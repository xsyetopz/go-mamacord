import {
	Button,
	Card,
	Code,
	Group,
	SimpleGrid,
	Stack,
	Text,
} from "@mantine/core";
import { IconCircleCheck, IconCircleX, IconRefresh } from "@tabler/icons-react";
import { PageHeader } from "../components/PageHeader";
import { CodeLine, SetupMessage, StatePanel } from "../components/StatePanel";
import { BoolStatusIconBadge } from "../components/StatusIconBadge";
import { type BootstrapState, localSetup } from "../dashboard";
import type { SetupStatus, StatusResponse } from "../types";

const TRAILING_SLASH_RE = /\/$/;

type Props = {
	bootstrap: BootstrapState;
	setupStatus: SetupStatus | null;
	status: StatusResponse | null;
	onRefresh: () => void;
	onLogin: () => void;
};

function ConnectionPanel({
	bootstrap,
	resolvedAdminAddr,
}: {
	bootstrap: BootstrapState;
	resolvedAdminAddr: string;
}) {
	const browserURL = window.location.origin;
	const apiURL = `${browserURL.replace(TRAILING_SLASH_RE, "")}/api`;
	return (
		<StatePanel
			title="Connection"
			status={
				<BoolStatusIconBadge
					value={
						bootstrap.kind !== "offline" &&
						bootstrap.kind !== "invalid_api_base"
					}
					labelTrue="Available"
					labelFalse="Needs attention"
					colorTrue="success"
					colorFalse="warning"
				/>
			}
		>
			<CodeLine label="Browser URL" value={browserURL} />
			<CodeLine label="API URL" value={apiURL} />
			<CodeLine label="Expected local admin API" value={resolvedAdminAddr} />
			{bootstrap.kind === "invalid_api_base" ? (
				<SetupMessage title="Invalid dashboard URL">
					<Stack gap="sm">
						<Text size="sm">{bootstrap.message}</Text>
						<Button
							variant="default"
							onClick={() => {
								window.location.href = `http://${resolvedAdminAddr}/`;
							}}
						>
							Open admin dashboard
						</Button>
					</Stack>
				</SetupMessage>
			) : null}
			{bootstrap.kind === "offline" ? (
				<SetupMessage title="Admin API not reachable">
					{bootstrap.message}
				</SetupMessage>
			) : null}
		</StatePanel>
	);
}

function SignInPanel({
	setupStatus,
	resolvedOrigin,
	resolvedRedirect,
	resolvedInstallRedirect,
	bootstrap,
	onLogin,
}: {
	setupStatus: SetupStatus | null;
	resolvedOrigin: string;
	resolvedRedirect: string;
	resolvedInstallRedirect: string;
	bootstrap: BootstrapState;
	onLogin: () => void;
}) {
	return (
		<StatePanel
			title="Sign-in"
			status={
				<BoolStatusIconBadge
					value={setupStatus?.login_ready ?? false}
					labelTrue="Ready"
					labelFalse="Not ready"
				/>
			}
		>
			<CodeLine label="Dashboard origin" value={resolvedOrigin} />
			<CodeLine label="Login callback URL" value={resolvedRedirect} />
			<CodeLine
				label="Install callback URL (optional)"
				value={resolvedInstallRedirect}
			/>
			<Text size="sm" c="dimmed">
				If Discord says “invalid OAuth2 URL”, it usually means one of these URLs
				is not in the Developer Portal Redirect URI allowlist.
			</Text>
			<Stack gap="xs">
				<Group gap="xs">
					<BoolStatusIconBadge
						value={setupStatus?.has_client_id ?? false}
						labelTrue="Client ID present"
						labelFalse="Client ID missing"
						iconTrue={({ size }) => <IconCircleCheck size={size} />}
						iconFalse={({ size }) => <IconCircleX size={size} />}
						variant="light"
					/>
					<Text size="sm">Client ID</Text>
				</Group>
				<Group gap="xs">
					<BoolStatusIconBadge
						value={setupStatus?.has_client_secret ?? false}
						labelTrue="Client secret present"
						labelFalse="Client secret missing"
						iconTrue={({ size }) => <IconCircleCheck size={size} />}
						iconFalse={({ size }) => <IconCircleX size={size} />}
						variant="light"
					/>
					<Text size="sm">Client secret</Text>
				</Group>
				<Group gap="xs">
					<BoolStatusIconBadge
						value={setupStatus?.has_session_secret ?? false}
						labelTrue="Session secret present"
						labelFalse="Session secret missing"
						iconTrue={({ size }) => <IconCircleCheck size={size} />}
						iconFalse={({ size }) => <IconCircleX size={size} />}
						variant="light"
					/>
					<Text size="sm">Session secret</Text>
				</Group>
				<Group gap="xs">
					<BoolStatusIconBadge
						value={setupStatus?.owner_resolved ?? false}
						labelTrue="Owner resolved"
						labelFalse="Owner unresolved"
						iconTrue={({ size }) => <IconCircleCheck size={size} />}
						iconFalse={({ size }) => <IconCircleX size={size} />}
						variant="light"
					/>
					<Text size="sm">Owner account</Text>
				</Group>
			</Stack>
			<CodeLine
				label="Owner source"
				value={setupStatus?.owner_source || "unresolved"}
			/>
			{setupStatus?.effective_owner_user_id ? (
				<CodeLine
					label="Effective owner user ID"
					value={String(setupStatus.effective_owner_user_id)}
				/>
			) : null}
			{bootstrap.kind === "unauthenticated" ? (
				<Button onClick={onLogin}>Sign in with Discord</Button>
			) : null}
		</StatePanel>
	);
}

function RuntimePanel({
	setupStatus,
	status,
}: {
	setupStatus: SetupStatus | null;
	status: StatusResponse | null;
}) {
	const discordStartError = status?.snapshot.discord_start_error?.trim() || "";
	return (
		<StatePanel
			title="Runtime"
			status={
				<BoolStatusIconBadge
					value={setupStatus?.signing_configured ?? false}
					labelTrue="Signing ready"
					labelFalse="Signing optional"
				/>
			}
		>
			<Group gap="xs" align="center">
				<Text size="sm">Trusted keys</Text>
				<BoolStatusIconBadge
					value={setupStatus?.trusted_keys_configured ?? false}
					labelTrue="Configured"
					labelFalse="Not configured"
					variant="light"
				/>
			</Group>
			{discordStartError ? (
				<SetupMessage title="Discord connection problem">
					<Stack gap="xs">
						<Text size="sm">
							The admin API is up, but the bot failed to connect to Discord.
						</Text>
						<Code block={true}>{discordStartError}</Code>
					</Stack>
				</SetupMessage>
			) : null}
			{status ? (
				<>
					<CodeLine
						label="Bundled plugins path"
						value={status.config.bundled_plugins_dir}
					/>
					<CodeLine
						label="User plugins path"
						value={status.config.user_plugins_dir}
					/>
					<CodeLine label="Modules file" value={status.config.modules_file} />
					<CodeLine
						label="Permissions file"
						value={status.config.permissions_file}
					/>
					<CodeLine
						label="Trusted keys file"
						value={status.config.trusted_keys_file}
					/>
				</>
			) : null}
		</StatePanel>
	);
}

function LocalSetupCard({ setupStatus }: { setupStatus: SetupStatus | null }) {
	const hints = Array.isArray(setupStatus?.hints) ? setupStatus.hints : [];
	return (
		<Card className="panel-card" withBorder={true}>
			<Stack gap="sm">
				<Text fw={700}>Local setup</Text>
				<Text size="sm">Use these values for a normal local run:</Text>
				<Code
					block={true}
				>{`MAMACORD_ADMIN_ADDR=${localSetup.adminAddr}`}</Code>
				{hints.length > 0 ? (
					<Stack gap="xs">
						<Text size="sm" fw={600}>
							What still needs attention
						</Text>
						{hints.map((hint) => (
							<Text key={hint} size="sm">
								{hint}
							</Text>
						))}
					</Stack>
				) : (
					<Text size="sm">The dashboard setup looks complete.</Text>
				)}
			</Stack>
		</Card>
	);
}

export function SetupPage({
	bootstrap,
	setupStatus,
	status,
	onRefresh,
	onLogin,
}: Props) {
	const resolvedAdminAddr = setupStatus?.admin_addr || localSetup.adminAddr;
	const resolvedOrigin =
		setupStatus?.app_origin || `http://${resolvedAdminAddr}`;
	const resolvedRedirect =
		setupStatus?.redirect_url ||
		`${resolvedOrigin.replace(TRAILING_SLASH_RE, "")}/api/auth/callback`;
	const resolvedInstallRedirect =
		setupStatus?.install_redirect_url ||
		`${resolvedOrigin.replace(TRAILING_SLASH_RE, "")}/api/install/callback`;

	return (
		<Stack gap="lg">
			<PageHeader
				title="Setup"
				subtitle="Connection, sign-in, and local runtime checks."
				action={
					<Button
						variant="default"
						leftSection={<IconRefresh size={16} />}
						onClick={onRefresh}
					>
						Refresh
					</Button>
				}
			/>
			<SimpleGrid cols={{ base: 1, md: 3 }} spacing="md">
				<ConnectionPanel
					bootstrap={bootstrap}
					resolvedAdminAddr={resolvedAdminAddr}
				/>
				<SignInPanel
					setupStatus={setupStatus}
					resolvedOrigin={resolvedOrigin}
					resolvedRedirect={resolvedRedirect}
					resolvedInstallRedirect={resolvedInstallRedirect}
					bootstrap={bootstrap}
					onLogin={onLogin}
				/>
				<RuntimePanel setupStatus={setupStatus} status={status} />
			</SimpleGrid>
			<LocalSetupCard setupStatus={setupStatus} />
		</Stack>
	);
}
