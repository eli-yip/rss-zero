import react from "@vitejs/plugin-react";
import { visualizer } from "rollup-plugin-visualizer";
import { defineConfig } from "vite";
import tsconfigPaths from "vite-tsconfig-paths";

// https://vitejs.dev/config/
export default defineConfig({
	plugins: [
		react(),
		tsconfigPaths(),
		visualizer({
			open: true,
			gzipSize: true,
			filename: "dist/stats.html",
		}),
	],
	build: {
		cssCodeSplit: true,
	},
	base: "",
	server: {
		proxy: {
			"/api": {
				target: "http://localhost:8080",
				changeOrigin: true,
			},
		},
	},
});
