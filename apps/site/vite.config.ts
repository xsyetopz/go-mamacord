import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const base = process.env["MAMACORD_SITE_BASE"] ?? "/";

export default defineConfig({
	base,
	plugins: [react()],
	server: {
		host: "127.0.0.1",
		port: 4173,
		strictPort: true,
	},
});
