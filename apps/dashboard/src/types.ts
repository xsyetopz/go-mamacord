export type AuthMe = {
	authenticated: boolean;
	user: {
		id: string;
		username: string;
		name: string;
		avatar_url?: string;
	};
	is_owner: boolean;
	csrf_token: string;
};

export type SetupStatus = {
	admin_enabled: boolean;
	auth_configured: boolean;
	login_ready: boolean;
	owner_configured: boolean;
	owner_resolved: boolean;
	owner_source: string;
	effective_owner_user_id?: string;
	signing_configured: boolean;
	trusted_keys_configured: boolean;
	admin_addr: string;
	app_origin: string;
	redirect_url: string;
	install_redirect_url: string;
	has_client_id: boolean;
	has_client_secret: boolean;
	has_session_secret: boolean;
	hints: string[];
};

export type Snapshot = {
	ready: boolean;
	started_at: string;
	migration_version: number;
	prod_mode: boolean;
	discord_start_error?: string;
	module_count: number;
	enabled_module_count: number;
	plugin_count: number;
	enabled_plugin_count: number;
	builtin_command_count: number;
	slash_command_count: number;
	user_command_count: number;
	message_command_count: number;
	interactions_total: number;
	interaction_failures: number;
	plugin_failures: number;
	automation_failures: number;
	reminder_failures: number;
};

export type StatusResponse = {
	snapshot: Snapshot;
	build: {
		version: string;
		repository?: string;
		description?: string;
		developer_url?: string;
		support_server_url?: string;
		mascot_image_url?: string;
	};
	config: {
		storage_backend: string;
		storage_target: string;
		migrations_dir: string;
		locales_dir: string;
		bundled_plugins_dir: string;
		user_plugins_dir: string;
		permissions_file: string;
		modules_file: string;
		trusted_keys_file: string;
		ops_addr: string;
		admin_addr: string;
		runtime_roles: string[];
		dev_guild_id?: string;
		command_registration_mode: string;
		prod_mode: boolean;
		allow_unsigned_plugins: boolean;
	};
	setup: SetupStatus;
};

export type ModuleInfo = {
	id: string;
	name: string;
	kind: string;
	runtime: string;
	enabled: boolean;
	default_enabled: boolean;
	toggleable: boolean;
	signed: boolean;
	source: string;
	commands: string[];
};

export type PluginSummary = {
	id: string;
	name: string;
	version: string;
	commands: string[];
	loaded: boolean;
	signed: boolean;
	has_signature_file: boolean;
	plugin_root: string;
	bundled: boolean;
	provenance_kind: string;
	source_id?: string;
	git_revision?: string;
	signature_state?: string;
	local_modified: boolean;
	bundle_relative_dir?: string;
};

export type MigrationStatus = {
	current_version: number;
	applied: Array<{ version: number; name: string; kind: string }>;
	pending: Array<{ version: number; name: string; kind: string }>;
};

export type GuildSummary = {
	id: string;
	name: string;
	icon_url?: string;
	owner: boolean;
	can_manage: boolean;
	bot_installed: boolean;
};

export type GuildDashboard = {
	guild: GuildSummary;
	install_url: string;
	setup_checks: Array<{
		id: string;
		label: string;
		ok: boolean;
		message: string;
	}>;
	manager: PluginSection & {
		channel_count: number;
		role_count: number;
		emoji_count: number;
		sticker_count: number;
	};
	moderation: PluginSection & {
		warning_limit: number;
		timeout_threshold: number;
		timeout_minutes: number;
	};
	fun: PluginSection;
	info: PluginSection;
	wellness: PluginSection & {
		allow_channel_reminders: boolean;
		default_reminder_channel_id?: string;
	};
};

export type PluginSection = {
	id: string;
	name: string;
	enabled: boolean;
	global_enabled: boolean;
	commands: Array<{
		id: string;
		label: string;
		enabled: boolean;
	}>;
};

export type GuildChannelInfo = {
	id: string;
	name: string;
	type: string;
	parent_id?: string;
};

export type GuildRoleInfo = {
	id: string;
	name: string;
	color: number;
	position: number;
	managed: boolean;
	mentionable: boolean;
};

export type GuildMemberInfo = {
	user_id: string;
	username: string;
	display_name: string;
	avatar_url?: string;
	bot: boolean;
	joined_at?: number;
	role_ids: string[];
};

export type GuildEmojiInfo = {
	id: string;
	name: string;
	animated: boolean;
};

export type GuildStickerInfo = {
	id: string;
	name: string;
	description?: string;
	tags?: string;
};

export type WarningInfo = {
	id: string;
	user_id: string;
	moderator_id: string;
	reason: string;
	created_at: string;
};

export type GuildPluginConfig = {
	enabled: boolean;
	commands: Record<string, boolean>;
	warning_limit?: number;
	timeout_threshold?: number;
	timeout_minutes?: number;
	allow_channel_reminders?: boolean;
	default_reminder_channel_id?: string | undefined;
};
