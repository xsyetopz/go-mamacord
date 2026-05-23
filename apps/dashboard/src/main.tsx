import { colorSchemeManager, siteTheme } from "@mamacord/web-ui";
import { MantineProvider } from "@mantine/core";
import { Notifications } from "@mantine/notifications";
import React from "react";
import ReactDOM from "react-dom/client";
import { App } from "./App";
import { setAPIBase, setAPIBaseError } from "./api";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { DeveloperDetailsProvider } from "./developerDetails";
import "@mantine/core/styles.css";
import "@mantine/notifications/styles.css";
import "@mamacord/web-ui/styles.css";
import "./styles.css";

const root = document.getElementById("root");
if (!root) {
	throw new Error('Missing root element with id="root".');
}
// TS control-flow narrowing does not always carry into nested async functions.
const rootEl = root;

const TRAILING_SLASH_RE = /\/$/;

async function loadDashboardConfig() {
	try {
		const resp = await fetch("/config.json", { cache: "no-store" });
		if (resp.status === 404) {
			return;
		}
		if (!resp.ok) {
			setAPIBaseError(`Could not load dashboard config (HTTP ${resp.status}).`);
			return;
		}
		const payload = (await resp.json()) as { api_origin?: unknown };
		const raw = String(payload.api_origin ?? "")
			.trim()
			.replace(TRAILING_SLASH_RE, "");
		if (raw === "") {
			return;
		}
		let url: URL;
		try {
			url = new URL(raw);
		} catch {
			setAPIBaseError(
				"Invalid dashboard config: api_origin must be a full URL.",
			);
			return;
		}
		if (url.protocol !== "http:" && url.protocol !== "https:") {
			setAPIBaseError("Invalid dashboard config: api_origin must be http(s).");
			return;
		}
		if (url.pathname !== "/" || url.search !== "" || url.hash !== "") {
			setAPIBaseError(
				"Invalid dashboard config: api_origin must be an origin only.",
			);
			return;
		}
		setAPIBase(url.origin);
	} catch {
		setAPIBaseError("Could not load dashboard config.");
	}
}

async function bootstrap() {
	await loadDashboardConfig();
	ReactDOM.createRoot(rootEl).render(
		<React.StrictMode>
			<MantineProvider
				theme={siteTheme}
				colorSchemeManager={colorSchemeManager}
				defaultColorScheme="auto"
			>
				<Notifications />
				<DeveloperDetailsProvider>
					<ErrorBoundary>
						<App />
					</ErrorBoundary>
				</DeveloperDetailsProvider>
			</MantineProvider>
		</React.StrictMode>,
	);
}

bootstrap().catch(() => {
	setAPIBaseError("Dashboard failed to start.");
});
