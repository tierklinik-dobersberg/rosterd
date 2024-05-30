import { resolve } from 'path';
import { defineConfig } from 'vite'

export default defineConfig({
    build: {
        rollupOptions: {
            input: {
                roster: resolve(__dirname, 'roster.html')
            }
        }
    }
})