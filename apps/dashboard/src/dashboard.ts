export type OwnerViewKey =
	| "overview"
	| "modules"
	| "plugins"
	| "create-plugin"
	| "setup"
	| "migrations";

export type AppRoute =
	| { kind: "home" }
	| { kind: "servers" }
	| { kind: "server"; guildID: string }
	| { kind: "owner"; view: OwnerViewKey };

export type PermissionsShape = {
	storage: Record<string, boolean>;
	discord: Record<string, boolean>;
	network: Record<string, boolean>;
	resources: Record<string, boolean>;
	automation: {
		jobs: boolean;
		events: Record<string, boolean>;
	};
};

export type BootstrapState =
	| { kind: "checking" }
	| { kind: "invalid_api_base"; message: string }
	| { kind: "offline"; message: string }
	| { kind: "unauthenticated" }
	| { kind: "ready" };

export type ScaffoldState = {
	id: string;
	name: string;
	version: string;
	locale: string;
	command_name: string;
	command_description: string;
	response_message: string;
	network_hosts: string;
	sign: boolean;
};

export const emptyPermissions: PermissionsShape = {
	storage: {
		kv: false,
		user_settings: false,
		checkins: false,
		reminders: false,
		warnings: false,
		audit: false,
	},
	discord: {
		users: false,
		guilds: false,
		channels: false,
		messages: false,
		reactions: false,
		members: false,
		roles: false,
		threads: false,
		invites: false,
		webhooks: false,
		emojis: false,
		stickers: false,
	},
	network: {
		http: false,
	},
	resources: {
		read: false,
	},
	automation: {
		jobs: false,
		events: {
			member_join_leave: false,
			moderation: false,
		},
	},
};

export const emptyScaffold: ScaffoldState = {
	id: "",
	name: "",
	version: "0.1.0",
	locale: "en-US",
	command_name: "",
	command_description: "",
	response_message: "",
	network_hosts: "",
	sign: false,
};

export const localSetup = {
	adminAddr: "127.0.0.1:8081",
};

const LEADING_HASH_RE = /^#/;

export function cloneEmptyPermissions(): PermissionsShape {
	return {
		storage: { ...emptyPermissions.storage },
		discord: { ...emptyPermissions.discord },
		network: { ...emptyPermissions.network },
		resources: { ...emptyPermissions.resources },
		automation: {
			jobs: emptyPermissions.automation.jobs,
			events: { ...emptyPermissions.automation.events },
		},
	};
}

export function parseRoute(hash: string): AppRoute {
	const clean = hash.replace(LEADING_HASH_RE, "").trim() || "/";
	const segments = clean.split("/").filter(Boolean);

	if (segments.length === 0) {
		return { kind: "home" };
	}
	if (segments[0] === "servers" && segments.length === 1) {
		return { kind: "servers" };
	}
	if (segments[0] === "servers" && segments[1]) {
		return { kind: "server", guildID: segments[1] };
	}
	if (segments[0] === "owner") {
		const view = segments[1] as OwnerViewKey | undefined;
		switch (view) {
			case "modules":
			case "plugins":
			case "create-plugin":
			case "setup":
			case "migrations":
			case "overview":
				return { kind: "owner", view };
			default:
				return { kind: "owner", view: "overview" };
		}
	}
	return { kind: "home" };
}

export function routeHash(route: AppRoute): string {
	switch (route.kind) {
		case "home":
			return "#/";
		case "servers":
			return "#/servers";
		case "server":
			return `#/servers/${route.guildID}`;
		case "owner":
			return route.view === "overview" ? "#/owner" : `#/owner/${route.view}`;
		default:
			return "#/";
	}
}
