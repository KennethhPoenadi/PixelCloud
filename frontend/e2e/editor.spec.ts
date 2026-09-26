import { expect, test } from '@playwright/test'
import { fileURLToPath } from 'node:url'

const sample = fileURLToPath(new URL('./fixtures/sample.jpg', import.meta.url))

test('register → upload → apply preset → download', async ({ page }) => {
  const email = `e2e-${Date.now()}@example.com`

  await page.goto('/register')
  await page.getByLabel('Email').fill(email)
  await page.getByLabel('Password').fill('password123')
  await page.getByRole('button', { name: 'Daftar' }).click()
  await expect(page).toHaveURL(/\/gallery/)

  await page.getByTestId('file-input').setInputFiles(sample)
  const thumb = page.getByRole('img', { name: 'sample.jpg' })
  await expect(thumb).toBeVisible()
  await thumb.click()
  await expect(page).toHaveURL(/\/editor\//)

  const vintage = page.getByRole('button', { name: 'Vintage' })
  await vintage.click()
  await expect(vintage).toHaveAttribute('aria-pressed', 'true')

  await page.getByRole('button', { name: /^Export/ }).click()
  const download = page.waitForEvent('download', { timeout: 150_000 })
  await page.getByRole('button', { name: 'Render & Download' }).click()
  expect((await download).suggestedFilename()).toBe('sample-pixelcloud.jpg')
  await expect(page.getByText('Siap diunduh')).toBeVisible()
})
