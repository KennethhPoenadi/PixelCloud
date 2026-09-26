//go:build integration

// Package integration runs the real HTTP flow against a running stack
// (Nginx → API → Postgres/Redis/MinIO → worker). Start it with `make up`, then:
//
//	PIXELCLOUD_URL=http://localhost:8080 go test -tags integration ./test/integration/
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png" // decode the PNG result
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"
)

var base = func() string {
	if u := os.Getenv("PIXELCLOUD_URL"); u != "" {
		return u + "/api/v1"
	}
	return "http://localhost:8080/api/v1"
}()

type client struct {
	t     *testing.T
	token string
}

func (c *client) do(method, path string, body io.Reader, contentType string, out any) int {
	c.t.Helper()
	req, err := http.NewRequest(method, base+path, body)
	if err != nil {
		c.t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			c.t.Fatalf("%s %s: decode %q: %v", method, path, data, err)
		}
	}
	return res.StatusCode
}

func (c *client) json(method, path string, in, out any) int {
	c.t.Helper()
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	return c.do(method, path, body, "application/json", out)
}

func sampleJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func (c *client) upload(name string, data []byte, out any) int {
	c.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", name)
	_, _ = fw.Write(data)
	_ = mw.Close()
	return c.do(http.MethodPost, "/images", &buf, mw.FormDataContentType(), out)
}

// viaBase sends a presigned URL through the same entrypoint as the API while
// keeping the Host header it was signed for (lets the test run in a container).
func viaBase(t *testing.T, raw string) *http.Request {
	t.Helper()
	signed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	entry, _ := url.Parse(base)
	host := signed.Host
	signed.Scheme, signed.Host = entry.Scheme, entry.Host
	req, _ := http.NewRequest(http.MethodGet, signed.String(), nil)
	req.Host = host
	return req
}

type apiError struct {
	Error struct{ Code, Message string } `json:"error"`
}

func TestFullFlow(t *testing.T) {
	c := &client{t: t}
	email := fmt.Sprintf("it-%d@example.com", time.Now().UnixNano())

	var session struct{ Token string }
	if s := c.json("POST", "/auth/register", map[string]string{"email": email, "password": "password123"}, &session); s != 201 {
		t.Fatalf("register: %d", s)
	}
	var e apiError
	if s := c.json("POST", "/auth/register", map[string]string{"email": email, "password": "password123"}, &e); s != 409 || e.Error.Code != "CONFLICT" {
		t.Fatalf("duplicate register: %d %+v", s, e)
	}
	if s := c.json("GET", "/me", nil, &e); s != 401 {
		t.Fatalf("/me without token: %d", s)
	}
	c.token = session.Token

	var img struct{ ID string }
	if s := c.upload("it.jpg", sampleJPEG(t, 64, 48), &img); s != 201 {
		t.Fatalf("upload: %d", s)
	}
	if s := c.upload("big.jpg", sampleJPEG(t, 2100, 10), &e); s != 413 || e.Error.Code != "FILE_TOO_LARGE" {
		t.Fatalf("free plan resolution limit: %d %+v", s, e)
	}
	if s := c.upload("x.txt", []byte("not an image at all"), &e); s != 415 {
		t.Fatalf("unsupported: %d", s)
	}

	pipeline := map[string]any{"version": 1, "operations": []map[string]any{
		{"op": "preset", "name": "noir"}, {"op": "rotate", "angle": 90},
	}}
	if s := c.json("POST", "/jobs", map[string]any{"image_id": img.ID, "pipeline": map[string]any{
		"version": 1, "operations": []map[string]any{{"op": "nope"}},
	}}, &e); s != 422 || e.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("invalid pipeline: %d %+v", s, e)
	}

	var job struct {
		ID, Status  string
		WorkerID    *string `json:"worker_id"`
		DownloadURL *string `json:"download_url"`
	}
	if s := c.json("POST", "/jobs", map[string]any{"image_id": img.ID, "pipeline": pipeline, "output_format": "png"}, &job); s != 202 {
		t.Fatalf("create job: %d", s)
	}
	deadline := time.Now().Add(2 * time.Minute)
	for job.Status != "done" && job.Status != "failed" && time.Now().Before(deadline) {
		time.Sleep(time.Second)
		c.json("GET", "/jobs/"+job.ID, nil, &job)
	}
	if job.Status != "done" || job.DownloadURL == nil || job.WorkerID == nil {
		t.Fatalf("job did not finish: %+v", job)
	}
	res, err := http.DefaultClient.Do(viaBase(t, *job.DownloadURL))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	out, _, err := image.DecodeConfig(res.Body)
	if err != nil || out.Width != 48 || out.Height != 64 {
		t.Fatalf("result should be a rotated 48x64 image: %+v %v", out, err)
	}

	var preset struct{ ID string }
	if s := c.json("POST", "/presets", map[string]any{"name": "mine", "pipeline": pipeline}, &preset); s != 201 {
		t.Fatalf("create preset: %d", s)
	}
	if s := c.json("POST", "/presets", map[string]any{"name": "mine", "pipeline": pipeline}, &e); s != 409 {
		t.Fatalf("duplicate preset: %d", s)
	}
	if s := c.json("DELETE", "/presets/"+preset.ID, nil, nil); s != 204 {
		t.Fatalf("delete preset: %d", s)
	}

	ids := make([]string, 6)
	for i := range ids {
		ids[i] = img.ID
	}
	ids[5] = "00000000-0000-0000-0000-000000000001"
	if s := c.json("POST", "/batches", map[string]any{"image_ids": ids, "pipeline": pipeline}, &e); s != 404 {
		t.Fatalf("batch with foreign image: %d %+v", s, e)
	}
	if s := c.json("GET", "/api-keys", nil, &e); s != 403 {
		t.Fatalf("api keys on free plan: %d", s)
	}

	if s := c.json("DELETE", "/images/"+img.ID, nil, nil); s != 204 {
		t.Fatalf("delete image: %d", s)
	}
	if s := c.json("GET", "/jobs/"+job.ID, nil, &e); s != 404 {
		t.Fatalf("jobs of a deleted image must be gone: %d", s)
	}
}
