import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api, type Image } from '@/lib/api'
import { errorMessage } from '@/lib/errors'

const CONCURRENCY = 3
const ACCEPTED = ['image/jpeg', 'image/png', 'image/webp']

/** Upload many files, a few at a time; one toast summarises the result. */
export function useUploads() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (files: File[]) => {
      const valid = files.filter((f) => ACCEPTED.includes(f.type))
      const skipped = files.length - valid.length
      const uploaded: Image[] = []
      const failures: string[] = []
      const queue = [...valid]
      const workers = Array.from({ length: Math.min(CONCURRENCY, queue.length) }, async () => {
        for (let file = queue.shift(); file; file = queue.shift()) {
          try {
            uploaded.push(await api.uploadImage(file))
          } catch (err) {
            failures.push(`${file.name}: ${errorMessage(err)}`)
          }
        }
      })
      await Promise.all(workers)
      return { uploaded, failures, skipped }
    },
    onMutate: (files) => toast.loading(`Mengunggah ${files.length} foto…`, { id: 'upload' }),
    onSuccess: ({ uploaded, failures, skipped }) => {
      if (uploaded.length) toast.success(`${uploaded.length} foto terunggah`, { id: 'upload' })
      else toast.dismiss('upload')
      if (skipped) toast.error(`${skipped} file dilewati: hanya JPEG, PNG, atau WebP.`)
      for (const f of failures.slice(0, 3)) toast.error(f)
    },
    onError: (err) => toast.error(errorMessage(err), { id: 'upload' }),
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: ['images'] })
      void queryClient.invalidateQueries({ queryKey: ['me'] })
    },
  })
}
