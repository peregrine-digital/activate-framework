import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import tailwindcss from "@tailwindcss/vite";
import { resolve } from "path";

export default defineConfig({
  plugins: [svelte(), tailwindcss()],
  resolve: {
    alias: {
      // Import shared UI components from the ui/ package
      "$lib": resolve(__dirname, "../../ui/src/lib"),
    },
  },
});
