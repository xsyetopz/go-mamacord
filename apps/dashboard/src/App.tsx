import {
	ActionIcon,
	AppShell,
	Avatar,
	Badge,
	Box,
	Burger,
	Button,
	Card,
	Divider,
	Drawer,
	Group,
	Loader,
	Menu,
	NavLink,
	Stack,
	Text,
	Title,
	useMantineColorScheme,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { notifications } from "@mantine/notifications";
import {
	IconBolt,
	IconCode,
	IconDeviceDesktop,
	IconLayoutDashboard,
	IconMoon,
	IconPlugConnected,
	IconPlus,
	IconServer,
	IconSun,
	IconTool,
} from "@tabler/icons-react";
import type { Dispatch, ReactNode, SetStateAction } from "react";
import {
	lazy,
	Suspense,
	useCallback,
	useEffect,
	useRef,
	useState,
} from "react";
import { APIError, apiBaseError, get, post } from "./api";
import {
	type AppRoute,
	type BootstrapState,
	cloneEmptyPermissions,
	emptyScaffold,
	type OwnerViewKey,
	type PermissionsShape,
	parseRoute,
	routeHash,
	type ScaffoldState,
} from "./dashboard";
import { useDeveloperDetails } from "./developerDetails";
import type {
	AuthMe,
	GuildDashboard,
	GuildSummary,
	MigrationStatus,
	ModuleInfo,
	PluginSummary,
	SetupStatus,
	StatusResponse,
} from "./types";

const CreatePluginPage = lazy(() =>
	import("./pages/CreatePluginPage").then((m) => ({
		default: m.CreatePluginPage,
	})),
);
const HomePage = lazy(() =>
	import("./pages/HomePage").then((m) => ({ default: m.HomePage })),
);
const MigrationsPage = lazy(() =>
	import("./pages/MigrationsPage").then((m) => ({ default: m.MigrationsPage })),
);
const ModulesPage = lazy(() =>
	import("./pages/ModulesPage").then((m) => ({ default: m.ModulesPage })),
);
const OverviewPage = lazy(() =>
	import("./pages/OverviewPage").then((m) => ({ default: m.OverviewPage })),
);
const PluginsPage = lazy(() =>
	import("./pages/PluginsPage").then((m) => ({ default: m.PluginsPage })),
);
const ServerDashboardPage = lazy(() =>
	import("./pages/ServerDashboardPage").then((m) => ({
		default: m.ServerDashboardPage,
	})),
);
const ServersPage = lazy(() =>
	import("./pages/ServersPage").then((m) => ({ default: m.ServersPage })),
);
const SetupPage = lazy(() =>
	import("./pages/SetupPage").then((m) => ({ default: m.SetupPage })),
);

const OWNER_NAV_ITEMS = [
	{ key: "overview", label: "Overview", icon: IconLayoutDashboard },
	{ key: "modules", label: "Modules", icon: IconBolt },
	{ key: "plugins", label: "Plugins", icon: IconPlugConnected },
	{ key: "create-plugin", label: "Create plugin", icon: IconPlus },
	{ key: "setup", label: "Setup", icon: IconTool },
	{ key: "migrations", label: "Migrations", icon: IconServer },
] satisfies Array<{
	key: OwnerViewKey;
	label: string;
	icon: typeof IconLayoutDashboard;
}>;

function isUnauthorized(error: unknown): error is APIError {
	return error instanceof APIError && error.status === 401;
}

function setupBootstrapStateFromError(
	error: unknown,
	message: string,
): Extract<BootstrapState, { kind: "offline" | "invalid_api_base" }> {
	if (error instanceof APIError && error.status === -1) {
		return { kind: "invalid_api_base", message };
	}
	return { kind: "offline", message };
}

function currentHashRoute(): AppRoute {
	return parseRoute(window.location.hash);
}

function navigate(route: AppRoute) {
	const next = routeHash(route);
	if (window.location.hash !== next) {
		window.location.hash = next;
	}
}

function notifyAsyncError(title: string, error: unknown) {
	notifications.show({
		color: "red",
		title,
		message: error instanceof Error ? error.message : "Unknown error",
	});
}

function ThemeMenu() {
	const { colorScheme, setColorScheme } = useMantineColorScheme();
	const { enabled: devDetailsEnabled, toggle: toggleDevDetails } =
		useDeveloperDetails();
	const current = colorScheme ?? "auto";
	const icon =
		current === "dark" ? (
			<IconMoon size={16} />
		) : current === "light" ? (
			<IconSun size={16} />
		) : (
			<IconDeviceDesktop size={16} />
		);

	return (
		<Menu position="bottom-end" withinPortal={true}>
			<Menu.Target>
				<ActionIcon
					variant="subtle"
					aria-label="Theme"
					title="Theme"
					radius="md"
				>
					{icon}
				</ActionIcon>
			</Menu.Target>
			<Menu.Dropdown>
				<Menu.Item
					leftSection={<IconDeviceDesktop size={16} />}
					onClick={() => setColorScheme("auto")}
					data-active={current === "auto" ? "true" : undefined}
				>
					Auto
				</Menu.Item>
				<Menu.Item
					leftSection={<IconSun size={16} />}
					onClick={() => setColorScheme("light")}
					data-active={current === "light" ? "true" : undefined}
				>
					Light
				</Menu.Item>
				<Menu.Item
					leftSection={<IconMoon size={16} />}
					onClick={() => setColorScheme("dark")}
					data-active={current === "dark" ? "true" : undefined}
				>
					Dark
				</Menu.Item>
				<Menu.Divider />
				<Menu.Label>Developer</Menu.Label>
				<Menu.Item
					leftSection={<IconCode size={16} />}
					onClick={toggleDevDetails}
				>
					Developer details: {devDetailsEnabled ? "On" : "Off"}
				</Menu.Item>
			</Menu.Dropdown>
		</Menu>
	);
}

function fireAndNotify(title: string, promise: Promise<unknown>) {
	promise.catch((error: unknown) => {
		notifyAsyncError(title, error);
	});
}

function SetupContainer({
	bootstrap,
	setupStatus,
	ownerStatus,
	onRefresh,
	onLogin,
}: {
	bootstrap: BootstrapState;
	setupStatus: SetupStatus | null;
	ownerStatus: StatusResponse | null;
	onRefresh: () => void;
	onLogin: () => void;
}) {
	return (
		<Box maw={1120} mx="auto" px="md" py="xl">
			<Suspense
				fallback={
					<Group justify="center" mt={80}>
						<Loader color="brand" />
					</Group>
				}
			>
				<SetupPage
					bootstrap={bootstrap}
					setupStatus={setupStatus}
					status={ownerStatus}
					onRefresh={onRefresh}
					onLogin={onLogin}
				/>
			</Suspense>
		</Box>
	);
}

function OwnerShell({
	me,
	view,
	bootstrap,
	setupStatus,
	ownerStatus,
	modules,
	plugins,
	migrationStatus,
	busy,
	scaffold,
	scaffoldPermissions,
	signingConfigured,
	onNavigate,
	onLogout,
	onLogin,
	onRefreshBootstrap,
	runAction,
	updateScaffold,
	togglePermission,
	setScaffoldPermissions,
	resetScaffold,
	csrfToken,
}: {
	me: AuthMe;
	view: OwnerViewKey;
	bootstrap: BootstrapState;
	setupStatus: SetupStatus | null;
	ownerStatus: StatusResponse | null;
	modules: ModuleInfo[];
	plugins: PluginSummary[];
	migrationStatus: MigrationStatus | null;
	busy: string | null;
	scaffold: ScaffoldState;
	scaffoldPermissions: PermissionsShape;
	signingConfigured: boolean;
	onNavigate: (route: AppRoute) => void;
	onLogout: () => Promise<void>;
	onLogin: () => void;
	onRefreshBootstrap: () => void;
	runAction: <T>(
		key: string,
		action: () => Promise<T>,
		message?: string,
	) => Promise<void>;
	updateScaffold: (field: keyof ScaffoldState, value: string | boolean) => void;
	togglePermission: (
		section: keyof PermissionsShape,
		key: string,
		value: boolean,
	) => void;
	setScaffoldPermissions: Dispatch<SetStateAction<PermissionsShape>>;
	resetScaffold: () => void;
	csrfToken: string;
}) {
	const [navOpened, nav] = useDisclosure(false);

	let ownerContent = ownerStatus ? (
		<OverviewPage status={ownerStatus} />
	) : (
		<Loader color="brand" />
	);

	switch (view) {
		case "modules":
			ownerContent = (
				<ModulesPage
					modules={modules}
					busy={busy}
					onReload={() =>
						runAction(
							"modules:reload",
							() => post("/api/owner/modules/reload", {}, csrfToken),
							"Modules reloaded.",
						)
					}
					onEnable={(moduleID) =>
						runAction(
							`module:enable:${moduleID}`,
							() =>
								post(
									"/api/owner/modules/set",
									{ module_id: moduleID, enabled: true },
									csrfToken,
								),
							"Module enabled.",
						)
					}
					onDisable={(moduleID) =>
						runAction(
							`module:disable:${moduleID}`,
							() =>
								post(
									"/api/owner/modules/set",
									{ module_id: moduleID, enabled: false },
									csrfToken,
								),
							"Module disabled.",
						)
					}
					onReset={(moduleID) =>
						runAction(
							`module:reset:${moduleID}`,
							() =>
								post(
									"/api/owner/modules/reset",
									{ module_id: moduleID },
									csrfToken,
								),
							"Module reset.",
						)
					}
				/>
			);
			break;
		case "plugins":
			ownerContent = (
				<PluginsPage
					plugins={plugins}
					busy={busy}
					signingConfigured={signingConfigured}
					onReload={() =>
						runAction(
							"plugins:reload",
							() => post("/api/owner/plugins/reload", {}, csrfToken),
							"Plugins reloaded.",
						)
					}
					onCreatePlugin={() =>
						onNavigate({ kind: "owner", view: "create-plugin" })
					}
					onSignPlugin={(pluginID) =>
						runAction(
							`plugin:sign:${pluginID}`,
							() =>
								post(
									"/api/owner/plugins/sign",
									{ plugin_id: pluginID },
									csrfToken,
								),
							"Plugin signed.",
						)
					}
				/>
			);
			break;
		case "create-plugin":
			ownerContent = (
				<CreatePluginPage
					scaffold={scaffold}
					permissions={scaffoldPermissions}
					busy={busy}
					signingConfigured={signingConfigured}
					onBack={() => onNavigate({ kind: "owner", view: "plugins" })}
					onSubmit={() =>
						runAction(
							"plugins:scaffold",
							async () => {
								await post(
									"/api/owner/plugins/scaffold",
									{
										...scaffold,
										permissions: scaffoldPermissions,
									},
									csrfToken,
								);
								resetScaffold();
								onNavigate({ kind: "owner", view: "plugins" });
							},
							"Plugin created.",
						)
					}
					onFieldChange={updateScaffold}
					onPermissionToggle={togglePermission}
					onJobsToggle={(value) =>
						setScaffoldPermissions((current) => ({
							...current,
							automation: {
								...current.automation,
								jobs: value,
							},
						}))
					}
				/>
			);
			break;
		case "setup":
			ownerContent = (
				<SetupPage
					bootstrap={bootstrap}
					setupStatus={setupStatus}
					status={ownerStatus}
					onRefresh={onRefreshBootstrap}
					onLogin={onLogin}
				/>
			);
			break;
		case "migrations":
			ownerContent = (
				<MigrationsPage migrationStatus={migrationStatus} />
			);
			break;
		default:
			ownerContent = ownerStatus ? (
				<OverviewPage status={ownerStatus} />
			) : (
				<Loader color="brand" />
			);
			break;
	}

	return (
		<AppShell
			header={{ height: 68 }}
			navbar={{
				width: 280,
				breakpoint: "sm",
				collapsed: { mobile: !navOpened },
			}}
			padding="lg"
		>
			<AppShell.Header className="owner-header">
				<Group justify="space-between" h="100%" px="lg">
					<Group gap="sm">
						<Burger
							opened={navOpened}
							onClick={nav.toggle}
							hiddenFrom="sm"
							aria-label="Toggle navigation"
						/>
						<Text fw={800}>mamacord</Text>
						<Badge variant="light" color="brand">
							Owner
						</Badge>
					</Group>
					<Group gap="sm">
						<ThemeMenu />
						<Button
							variant="subtle"
							onClick={() => {
								nav.close();
								onNavigate({ kind: "servers" });
							}}
						>
							Servers
						</Button>
						<Button
							variant="default"
							onClick={() => {
								nav.close();
								onLogout().catch(() => undefined);
							}}
						>
							Sign out
						</Button>
					</Group>
				</Group>
			</AppShell.Header>
			<AppShell.Navbar p="md">
				<Stack gap="sm" h="100%">
					<Group gap="sm">
						<Avatar
							src={me.user.avatar_url ?? null}
							radius="md"
							color="brand"
							name={me.user.name}
						/>
						<Stack gap={0}>
							<Text fw={600}>{me.user.name}</Text>
							<Text size="xs" c="dimmed">
								{me.user.username}
							</Text>
						</Stack>
					</Group>
					<Divider />
					<Stack gap={4}>
						{OWNER_NAV_ITEMS.map((item) => (
							<NavLink
								key={item.key}
								active={view === item.key}
								label={item.label}
								leftSection={<item.icon size={16} />}
								onClick={() => {
									nav.close();
									onNavigate({ kind: "owner", view: item.key });
								}}
							/>
						))}
					</Stack>
				</Stack>
			</AppShell.Navbar>
			<AppShell.Main>
				<Suspense
					fallback={
						<Group justify="center" mt={80}>
							<Loader color="brand" />
						</Group>
					}
				>
					{ownerContent}
				</Suspense>
			</AppShell.Main>
		</AppShell>
	);
}

function NotOwner({ onGoServers }: { onGoServers: () => void }) {
	return (
		<Box maw={1120} mx="auto" px="md" py="xl">
			<Card className="panel-card" withBorder={true}>
				<Stack gap="sm">
					<Title order={2}>Owner access required</Title>
					<Text size="sm" c="dimmed">
						This area is for bot-global controls. Use the server dashboard for
						normal server management.
					</Text>
					<Group>
						<Button variant="default" onClick={onGoServers}>
							Go to servers
						</Button>
					</Group>
				</Stack>
			</Card>
		</Box>
	);
}

function CheckingScreen() {
	return (
		<Box p="xl">
			<Group justify="center" mt={80}>
				<Loader color="brand" />
			</Group>
		</Box>
	);
}

function OwnerRouteView({
	route,
	bootstrap,
	me,
	setupStatus,
	ownerStatus,
	modules,
	plugins,
	migrationStatus,
	busy,
	scaffold,
	scaffoldPermissions,
	csrfToken,
	onNavigate,
	onLogin,
	onLogout,
	onRefreshBootstrap,
	runAction,
	updateScaffold,
	togglePermission,
	setScaffoldPermissions,
	resetScaffold,
}: {
	route: Extract<AppRoute, { kind: "owner" }>;
	bootstrap: BootstrapState;
	me: AuthMe | null;
	setupStatus: SetupStatus | null;
	ownerStatus: StatusResponse | null;
	modules: ModuleInfo[];
	plugins: PluginSummary[];
	migrationStatus: MigrationStatus | null;
	busy: string | null;
	scaffold: ScaffoldState;
	scaffoldPermissions: PermissionsShape;
	csrfToken: string;
	onNavigate: (route: AppRoute) => void;
	onLogin: () => void;
	onLogout: () => Promise<void>;
	onRefreshBootstrap: () => void;
	runAction: <T>(
		key: string,
		action: () => Promise<T>,
		message?: string,
	) => Promise<void>;
	updateScaffold: (field: keyof ScaffoldState, value: string | boolean) => void;
	togglePermission: (
		section: keyof PermissionsShape,
		key: string,
		value: boolean,
	) => void;
	setScaffoldPermissions: Dispatch<SetStateAction<PermissionsShape>>;
	resetScaffold: () => void;
}) {
	const signedIn = bootstrap.kind === "ready" && me !== null;

	if (!signedIn) {
		return (
			<SetupContainer
				bootstrap={bootstrap}
				setupStatus={setupStatus}
				ownerStatus={ownerStatus}
				onRefresh={onRefreshBootstrap}
				onLogin={onLogin}
			/>
		);
	}

	if (!me.is_owner) {
		return <NotOwner onGoServers={() => onNavigate({ kind: "servers" })} />;
	}

	return (
		<OwnerShell
			me={me}
			view={route.view}
			bootstrap={bootstrap}
			setupStatus={setupStatus}
			ownerStatus={ownerStatus}
			modules={modules}
			plugins={plugins}
			migrationStatus={migrationStatus}
			busy={busy}
			scaffold={scaffold}
			scaffoldPermissions={scaffoldPermissions}
			signingConfigured={setupStatus?.signing_configured ?? false}
			onNavigate={onNavigate}
			onLogout={onLogout}
			onLogin={onLogin}
			onRefreshBootstrap={onRefreshBootstrap}
			runAction={runAction}
			updateScaffold={updateScaffold}
			togglePermission={togglePermission}
			setScaffoldPermissions={setScaffoldPermissions}
			resetScaffold={resetScaffold}
			csrfToken={csrfToken}
		/>
	);
}

function PublicSiteView({
	route,
	me,
	setupStatus,
	guilds,
	guildDashboard,
	guildsLoading,
	guildDashboardLoading,
	csrfToken,
	onLogin,
	onLogoutClick,
	onNavigate,
	onRefreshGuilds,
	onRefreshGuildDashboard,
}: {
	route: Exclude<AppRoute, { kind: "owner" }>;
	me: AuthMe | null;
	setupStatus: SetupStatus | null;
	guilds: GuildSummary[];
	guildDashboard: GuildDashboard | null;
	guildsLoading: boolean;
	guildDashboardLoading: boolean;
	csrfToken: string;
	onLogin: () => void;
	onLogoutClick: () => void;
	onNavigate: (route: AppRoute) => void;
	onRefreshGuilds: () => void;
	onRefreshGuildDashboard: (guildID: string) => void;
}) {
	let publicContent: ReactNode;
	const [navOpened, nav] = useDisclosure(false);

	switch (route.kind) {
		case "servers":
			publicContent = (
				<ServersPage
					me={me}
					guilds={guilds}
					loading={guildsLoading}
					onLogin={onLogin}
					onRefresh={onRefreshGuilds}
					onInviteBot={() => {
						window.location.href = "/api/install/start";
					}}
					onOpenGuild={(guildID) =>
						onNavigate({ kind: "server", guildID: String(guildID) })
					}
				/>
			);
			break;
		case "server":
			publicContent = (
				<ServerDashboardPage
					me={me}
					guildDashboard={guildDashboard}
					guildID={route.guildID}
					csrfToken={csrfToken}
					loading={guildDashboardLoading}
					onLogin={onLogin}
					onBack={() => onNavigate({ kind: "servers" })}
					onRefresh={() => onRefreshGuildDashboard(route.guildID)}
					onInstall={(guildID) => {
						window.location.href = `/api/install/start?guild_id=${encodeURIComponent(
							guildID,
						)}`;
					}}
				/>
			);
			break;
		default:
			publicContent = (
				<HomePage
					me={me}
					setupStatus={setupStatus}
					onLogin={onLogin}
					onOpenServers={() => onNavigate({ kind: "servers" })}
				/>
			);
			break;
	}

	return (
		<Box className="site-shell">
			<Box component="header" className="site-header">
				<Drawer
					opened={navOpened}
					onClose={nav.close}
					title="Navigation"
					padding="md"
					size="xs"
					hiddenFrom="sm"
				>
					<Stack gap="xs">
						<Button
							variant="subtle"
							justify="flex-start"
							onClick={() => {
								nav.close();
								onNavigate({ kind: "home" });
							}}
						>
							Home
						</Button>
						<Button
							variant="subtle"
							justify="flex-start"
							onClick={() => {
								nav.close();
								onNavigate({ kind: "servers" });
							}}
						>
							Servers
						</Button>
						{me?.is_owner ? (
							<Button
								variant="subtle"
								justify="flex-start"
								onClick={() => {
									nav.close();
									onNavigate({ kind: "owner", view: "overview" });
								}}
							>
								Owner
							</Button>
						) : null}
						<Divider my="sm" />
						<Group justify="space-between">
							<Text fw={700}>Theme</Text>
							<ThemeMenu />
						</Group>
						<Divider my="sm" />
						{me ? (
							<Button
								variant="default"
								onClick={() => {
									nav.close();
									onLogoutClick();
								}}
							>
								Sign out
							</Button>
						) : (
							<Button
								onClick={() => {
									nav.close();
									onLogin();
								}}
							>
								Sign in with Discord
							</Button>
						)}
					</Stack>
				</Drawer>
				<Group justify="space-between" className="site-header-inner">
					<Group gap="lg" className="site-header-left">
						<Burger
							opened={navOpened}
							onClick={nav.toggle}
							hiddenFrom="sm"
							aria-label="Toggle navigation"
						/>
						<Button
							variant="subtle"
							className="brand-button"
							onClick={() => onNavigate({ kind: "home" })}
						>
							mamacord
						</Button>
						<Group gap="xs" className="site-nav" visibleFrom="sm">
							<Button
								variant="subtle"
								onClick={() => onNavigate({ kind: "home" })}
							>
								Home
							</Button>
							<Button
								variant="subtle"
								onClick={() => onNavigate({ kind: "servers" })}
							>
								Servers
							</Button>
							{me?.is_owner ? (
								<Button
									variant="subtle"
									onClick={() =>
										onNavigate({ kind: "owner", view: "overview" })
									}
								>
									Owner
								</Button>
							) : null}
						</Group>
					</Group>
					<Group gap="sm">
						<ThemeMenu />
						{me ? (
							<>
								<Group gap="sm" className="session-chip">
									<Avatar
										src={me.user.avatar_url ?? null}
										radius="xl"
										size={30}
										color="brand"
										name={me.user.name}
									/>
									<Stack gap={0}>
										<Text size="sm" fw={600}>
											{me.user.name}
										</Text>
										<Text size="xs" c="dimmed">
											{me.user.username}
										</Text>
									</Stack>
								</Group>
								<Button variant="default" onClick={onLogoutClick}>
									Sign out
								</Button>
							</>
						) : (
							<Button onClick={onLogin}>Sign in with Discord</Button>
						)}
					</Group>
				</Group>
			</Box>
			<Box component="main" className="site-main">
				<Suspense
					fallback={
						<Group justify="center" mt={80}>
							<Loader color="brand" />
						</Group>
					}
				>
					{publicContent}
				</Suspense>
			</Box>
			<Box component="footer" className="site-footer">
				<Text size="xs" c="dimmed">
					Theme uses IBM-style neutrals with Wong color-blind-safe status
					colors.
				</Text>
			</Box>
		</Box>
	);
}

function AppView({
	route,
	bootstrap,
	me,
	setupStatus,
	ownerStatus,
	modules,
	plugins,
	migrationStatus,
	guilds,
	guildDashboard,
	guildsLoading,
	guildDashboardLoading,
	busy,
	scaffold,
	scaffoldPermissions,
	csrfToken,
	onNavigate,
	onLogin,
	onLogout,
	onLogoutClick,
	onRefreshBootstrap,
	onRefreshGuilds,
	onRefreshGuildDashboard,
	runAction,
	updateScaffold,
	togglePermission,
	setScaffoldPermissions,
	resetScaffold,
}: {
	route: AppRoute;
	bootstrap: BootstrapState;
	me: AuthMe | null;
	setupStatus: SetupStatus | null;
	ownerStatus: StatusResponse | null;
	modules: ModuleInfo[];
	plugins: PluginSummary[];
	migrationStatus: MigrationStatus | null;
	guilds: GuildSummary[];
	guildDashboard: GuildDashboard | null;
	guildsLoading: boolean;
	guildDashboardLoading: boolean;
	busy: string | null;
	scaffold: ScaffoldState;
	scaffoldPermissions: PermissionsShape;
	csrfToken: string;
	onNavigate: (route: AppRoute) => void;
	onLogin: () => void;
	onLogout: () => Promise<void>;
	onLogoutClick: () => void;
	onRefreshBootstrap: () => void;
	onRefreshGuilds: () => void;
	onRefreshGuildDashboard: (guildID: string) => void;
	runAction: <T>(
		key: string,
		action: () => Promise<T>,
		message?: string,
	) => Promise<void>;
	updateScaffold: (field: keyof ScaffoldState, value: string | boolean) => void;
	togglePermission: (
		section: keyof PermissionsShape,
		key: string,
		value: boolean,
	) => void;
	setScaffoldPermissions: Dispatch<SetStateAction<PermissionsShape>>;
	resetScaffold: () => void;
}) {
	if (bootstrap.kind === "checking") {
		return <CheckingScreen />;
	}

	if (bootstrap.kind === "invalid_api_base" || bootstrap.kind === "offline") {
		return (
			<SetupContainer
				bootstrap={bootstrap}
				setupStatus={setupStatus}
				ownerStatus={ownerStatus}
				onRefresh={onRefreshBootstrap}
				onLogin={onLogin}
			/>
		);
	}

	if (route.kind === "owner") {
		return (
			<OwnerRouteView
				route={route}
				bootstrap={bootstrap}
				me={me}
				setupStatus={setupStatus}
				ownerStatus={ownerStatus}
				modules={modules}
				plugins={plugins}
				migrationStatus={migrationStatus}
				busy={busy}
				scaffold={scaffold}
				scaffoldPermissions={scaffoldPermissions}
				csrfToken={csrfToken}
				onNavigate={onNavigate}
				onLogin={onLogin}
				onLogout={onLogout}
				onRefreshBootstrap={onRefreshBootstrap}
				runAction={runAction}
				updateScaffold={updateScaffold}
				togglePermission={togglePermission}
				setScaffoldPermissions={setScaffoldPermissions}
				resetScaffold={resetScaffold}
			/>
		);
	}

	return (
		<PublicSiteView
			route={route}
			me={me}
			setupStatus={setupStatus}
			guilds={guilds}
			guildDashboard={guildDashboard}
			guildsLoading={guildsLoading}
			guildDashboardLoading={guildDashboardLoading}
			csrfToken={csrfToken}
			onLogin={onLogin}
			onLogoutClick={onLogoutClick}
			onNavigate={onNavigate}
			onRefreshGuilds={onRefreshGuilds}
			onRefreshGuildDashboard={onRefreshGuildDashboard}
		/>
	);
}

function useAutoRefreshForRoute({
	bootstrapKind,
	route,
	isOwner,
	refreshGuilds,
	refreshGuildDashboard,
	refreshOwnerData,
}: {
	bootstrapKind: BootstrapState["kind"];
	route: AppRoute;
	isOwner: boolean;
	refreshGuilds: () => Promise<void>;
	refreshGuildDashboard: (guildID: string) => Promise<void>;
	refreshOwnerData: () => Promise<void>;
}) {
	useEffect(() => {
		if (bootstrapKind !== "ready") {
			return;
		}
		if (route.kind === "servers") {
			fireAndNotify("Could not load servers", refreshGuilds());
			return;
		}
		if (route.kind === "server") {
			fireAndNotify(
				"Could not load server",
				refreshGuildDashboard(route.guildID),
			);
			return;
		}
		if (route.kind === "owner" && isOwner) {
			fireAndNotify("Could not load owner data", refreshOwnerData());
		}
	}, [
		bootstrapKind,
		route,
		isOwner,
		refreshGuildDashboard,
		refreshGuilds,
		refreshOwnerData,
	]);
}

async function bootstrapDashboardImpl({
	apiBaseErrorMessage,
	setBootstrap,
	setSetupStatus,
	setMe,
}: {
	apiBaseErrorMessage: string | null;
	setBootstrap: Dispatch<SetStateAction<BootstrapState>>;
	setSetupStatus: Dispatch<SetStateAction<SetupStatus | null>>;
	setMe: Dispatch<SetStateAction<AuthMe | null>>;
}) {
	if (apiBaseErrorMessage) {
		setBootstrap({ kind: "invalid_api_base", message: apiBaseErrorMessage });
		return;
	}
	try {
		const setup = await get<SetupStatus>("/api/setup");
		setSetupStatus(setup);
	} catch (error) {
		const message =
			error instanceof Error ? error.message : "Could not reach the admin API.";
		setBootstrap(setupBootstrapStateFromError(error, message));
		return;
	}

	try {
		const meResp = await get<AuthMe>("/api/auth/me");
		if (!meResp.authenticated) {
			setMe(null);
			setBootstrap({ kind: "unauthenticated" });
			return;
		}
		setMe(meResp);
		setBootstrap({ kind: "ready" });
	} catch (error) {
		if (isUnauthorized(error)) {
			setMe(null);
			setBootstrap({ kind: "unauthenticated" });
			return;
		}
		const message =
			error instanceof Error ? error.message : "Could not load session state.";
		setBootstrap({ kind: "offline", message });
	}
}

async function refreshCurrentRouteImpl({
	route,
	isOwner,
	refreshOwnerData,
	refreshGuilds,
	refreshGuildDashboard,
}: {
	route: AppRoute;
	isOwner: boolean;
	refreshOwnerData: () => Promise<void>;
	refreshGuilds: () => Promise<void>;
	refreshGuildDashboard: (guildID: string) => Promise<void>;
}) {
	if (route.kind === "owner" && isOwner) {
		await refreshOwnerData();
		return;
	}
	if (route.kind === "servers") {
		await refreshGuilds();
		return;
	}
	if (route.kind === "server") {
		await refreshGuildDashboard(route.guildID);
	}
}

async function runActionImpl<T>({
	key,
	action,
	message,
	setBusy,
	refreshCurrentRoute,
}: {
	key: string;
	action: () => Promise<T>;
	message: string | undefined;
	setBusy: Dispatch<SetStateAction<string | null>>;
	refreshCurrentRoute: () => Promise<void>;
}) {
	setBusy(key);
	try {
		await action();
		await refreshCurrentRoute();
		if (message) {
			notifications.show({ color: "teal", title: "Saved", message });
		}
	} catch (error) {
		notifications.show({
			color: "red",
			title: "Action failed",
			message: error instanceof Error ? error.message : "Unknown error",
		});
	} finally {
		setBusy(null);
	}
}

export function App() {
	const [route, setRoute] = useState<AppRoute>(() => currentHashRoute());
	const [bootstrap, setBootstrap] = useState<BootstrapState>({
		kind: "checking",
	});
	const [me, setMe] = useState<AuthMe | null>(null);
	const [setupStatus, setSetupStatus] = useState<SetupStatus | null>(null);

	const [ownerStatus, setOwnerStatus] = useState<StatusResponse | null>(null);
	const [modules, setModules] = useState<ModuleInfo[]>([]);
	const [plugins, setPlugins] = useState<PluginSummary[]>([]);
	const [migrationStatus, setMigrationStatus] =
		useState<MigrationStatus | null>(null);

	const [guilds, setGuilds] = useState<GuildSummary[]>([]);
	const [guildDashboard, setGuildDashboard] = useState<GuildDashboard | null>(
		null,
	);
	const [guildsLoading, setGuildsLoading] = useState(false);
	const [guildDashboardLoading, setGuildDashboardLoading] = useState(false);

	const [busy, setBusy] = useState<string | null>(null);
	const [scaffold, setScaffold] = useState<ScaffoldState>(emptyScaffold);
	const [scaffoldPermissions, setScaffoldPermissions] =
		useState<PermissionsShape>(cloneEmptyPermissions());
	const bootstrappedRef = useRef(false);

	const csrfToken = me?.csrf_token ?? "";

	useEffect(() => {
		const onHashChange = () => {
			setRoute(currentHashRoute());
		};
		window.addEventListener("hashchange", onHashChange);
		return () => window.removeEventListener("hashchange", onHashChange);
	}, []);

	const refreshOwnerData = useCallback(async () => {
		if (!me?.is_owner) {
			return;
		}
		const [setup, statusResp, modulesResp, pluginsResp, migrations] =
			await Promise.all([
				get<SetupStatus>("/api/setup"),
				get<StatusResponse>("/api/owner/status"),
				get<{ modules: ModuleInfo[] }>("/api/owner/modules"),
				get<{ plugins: PluginSummary[] }>("/api/owner/plugins"),
				get<MigrationStatus>("/api/owner/migrations/status"),
			]);
		setSetupStatus(setup);
		setOwnerStatus(statusResp);
		setModules(modulesResp.modules);
		setPlugins(pluginsResp.plugins);
		setMigrationStatus(migrations);
	}, [me?.is_owner]);

	const refreshGuilds = useCallback(async () => {
		if (!me) {
			setGuilds([]);
			return;
		}
		setGuildsLoading(true);
		try {
			const payload = await get<{ guilds: GuildSummary[] }>("/api/guilds");
			setGuilds(payload.guilds);
		} finally {
			setGuildsLoading(false);
		}
	}, [me]);

	const refreshGuildDashboard = useCallback(
		async (guildID: string) => {
			if (!me) {
				setGuildDashboard(null);
				return;
			}
			setGuildDashboardLoading(true);
			try {
				const payload = await get<GuildDashboard>(
					`/api/guilds/dashboard?guild_id=${encodeURIComponent(guildID)}`,
				);
				setGuildDashboard(payload);
			} finally {
				setGuildDashboardLoading(false);
			}
		},
		[me],
	);

	const bootstrapDashboard = useCallback(
		() =>
			bootstrapDashboardImpl({
				apiBaseErrorMessage: apiBaseError,
				setBootstrap,
				setSetupStatus,
				setMe,
			}),
		[],
	);

	useEffect(() => {
		if (bootstrappedRef.current) {
			return;
		}
		bootstrappedRef.current = true;
		bootstrapDashboard().catch(() => undefined);
	}, [bootstrapDashboard]);

	useAutoRefreshForRoute({
		bootstrapKind: bootstrap.kind,
		route,
		isOwner: me?.is_owner ?? false,
		refreshGuilds,
		refreshGuildDashboard,
		refreshOwnerData,
	});

	const refreshCurrentRoute = useCallback(
		() =>
			refreshCurrentRouteImpl({
				route,
				isOwner: me?.is_owner ?? false,
				refreshOwnerData,
				refreshGuilds,
				refreshGuildDashboard,
			}),
		[
			me?.is_owner,
			refreshGuildDashboard,
			refreshGuilds,
			refreshOwnerData,
			route,
		],
	);

	const runAction = useCallback(
		<T,>(key: string, action: () => Promise<T>, message?: string) =>
			runActionImpl({
				key,
				action,
				message,
				setBusy,
				refreshCurrentRoute,
			}),
		[refreshCurrentRoute],
	);

	function login() {
		window.location.href = "/api/auth/login";
	}

	async function logout() {
		setBusy("logout");
		try {
			await post("/api/auth/logout", {}, csrfToken);
			setMe(null);
			setOwnerStatus(null);
			setGuilds([]);
			setGuildDashboard(null);
			setBootstrap({ kind: "unauthenticated" });
			navigate({ kind: "home" });
			notifications.show({
				color: "teal",
				title: "Signed out",
				message: "Session closed.",
			});
		} catch (error) {
			notifications.show({
				color: "red",
				title: "Sign-out failed",
				message: error instanceof Error ? error.message : "Unknown error",
			});
		} finally {
			setBusy(null);
		}
	}

	function updateScaffold(field: keyof ScaffoldState, value: string | boolean) {
		setScaffold((current) => ({
			...current,
			[field]: value,
		}));
	}

	function togglePermission(
		section: keyof PermissionsShape,
		key: string,
		value: boolean,
	) {
		setScaffoldPermissions((current) => {
			if (section === "automation") {
				return {
					...current,
					automation: {
						...current.automation,
						events: {
							...current.automation.events,
							[key]: value,
						},
					},
				};
			}
			return {
				...current,
				[section]: {
					...current[section],
					[key]: value,
				},
			};
		});
	}

	const resetScaffold = () => {
		setScaffold(emptyScaffold);
		setScaffoldPermissions(cloneEmptyPermissions());
	};

	const onRefreshBootstrap = () => {
		bootstrapDashboard().catch(() => undefined);
	};

	const onLogoutClick = () => {
		logout().catch(() => undefined);
	};

	const onRefreshGuilds = () => {
		fireAndNotify("Could not load servers", refreshGuilds());
	};

	const onRefreshGuildDashboard = (guildID: string) => {
		fireAndNotify("Could not load server", refreshGuildDashboard(guildID));
	};

	return (
		<AppView
			route={route}
			bootstrap={bootstrap}
			me={me}
			setupStatus={setupStatus}
			ownerStatus={ownerStatus}
			modules={modules}
			plugins={plugins}
			migrationStatus={migrationStatus}
			guilds={guilds}
			guildDashboard={guildDashboard}
			guildsLoading={guildsLoading}
			guildDashboardLoading={guildDashboardLoading}
			busy={busy}
			scaffold={scaffold}
			scaffoldPermissions={scaffoldPermissions}
			csrfToken={csrfToken}
			onNavigate={navigate}
			onLogin={login}
			onLogout={logout}
			onLogoutClick={onLogoutClick}
			onRefreshBootstrap={onRefreshBootstrap}
			onRefreshGuilds={onRefreshGuilds}
			onRefreshGuildDashboard={onRefreshGuildDashboard}
			runAction={runAction}
			updateScaffold={updateScaffold}
			togglePermission={togglePermission}
			setScaffoldPermissions={setScaffoldPermissions}
			resetScaffold={resetScaffold}
		/>
	);
}
