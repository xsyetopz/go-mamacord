import {
	Button,
	Card,
	Checkbox,
	Code,
	Group,
	SimpleGrid,
	Stack,
	Text,
	TextInput,
} from "@mantine/core";
import { IconArrowLeft } from "@tabler/icons-react";
import { PageHeader } from "../components/PageHeader";
import type { PermissionsShape, ScaffoldState } from "../dashboard";

function scaffoldBundleRelativeDir(version: string): string {
	const rawVersion = version.trim() || "0.1.0";
	const cleaned = `manual-v${rawVersion}`
		.replace(/[^A-Za-z0-9._-]+/g, "-")
		.replace(/^[._-]+|[._-]+$/g, "");
	return `bundles/${cleaned || "manual"}`;
}

type Props = {
	scaffold: ScaffoldState;
	permissions: PermissionsShape;
	busy: string | null;
	signingConfigured: boolean;
	onBack: () => void;
	onSubmit: () => void;
	onFieldChange: (field: keyof ScaffoldState, value: string | boolean) => void;
	onPermissionToggle: (
		section: keyof PermissionsShape,
		key: string,
		value: boolean,
	) => void;
	onJobsToggle: (value: boolean) => void;
};

export function CreatePluginPage({
	scaffold,
	permissions,
	busy,
	signingConfigured,
	onBack,
	onSubmit,
	onFieldChange,
	onPermissionToggle,
	onJobsToggle,
}: Props) {
	const pluginID = scaffold.id.trim() || "sample";
	const locale = scaffold.locale.trim() || "en-US";
	const bundleDir = scaffoldBundleRelativeDir(scaffold.version);
	const files = [
		`plugins/${pluginID}/.mamacord-bundle.json`,
		`plugins/${pluginID}/${bundleDir}/plugin.json`,
		`plugins/${pluginID}/${bundleDir}/plugin.lua`,
		`plugins/${pluginID}/${bundleDir}/commands/hello.lua`,
		`plugins/${pluginID}/${bundleDir}/locales/${locale}/messages.json`,
	];

	return (
		<Stack gap="lg">
			<PageHeader
				title="Create plugin"
				subtitle="Generate a starter plugin under plugins/."
				action={
					<Button
						variant="default"
						leftSection={<IconArrowLeft size={16} />}
						onClick={onBack}
					>
						Back to plugins
					</Button>
				}
			/>
			<SimpleGrid cols={{ base: 1, md: 3 }} spacing="md">
				<Card className="panel-card" withBorder={true}>
					<Stack gap="md">
						<Text fw={700}>Plugin</Text>
						<TextInput
							label="Plugin ID"
							placeholder="sample"
							value={scaffold.id}
							onChange={(event) =>
								onFieldChange("id", event.currentTarget.value)
							}
						/>
						<TextInput
							label="Display name"
							placeholder="Sample"
							value={scaffold.name}
							onChange={(event) =>
								onFieldChange("name", event.currentTarget.value)
							}
						/>
						<TextInput
							label="Version"
							value={scaffold.version}
							onChange={(event) =>
								onFieldChange("version", event.currentTarget.value)
							}
						/>
						<TextInput
							label="Locale"
							value={scaffold.locale}
							onChange={(event) =>
								onFieldChange("locale", event.currentTarget.value)
							}
						/>
					</Stack>
				</Card>
				<Card className="panel-card" withBorder={true}>
					<Stack gap="md">
						<Text fw={700}>Starter command</Text>
						<TextInput
							label="Command name"
							placeholder="sample"
							value={scaffold.command_name}
							onChange={(event) =>
								onFieldChange("command_name", event.currentTarget.value)
							}
						/>
						<TextInput
							label="Command description"
							placeholder="Run the sample command"
							value={scaffold.command_description}
							onChange={(event) =>
								onFieldChange("command_description", event.currentTarget.value)
							}
						/>
						<TextInput
							label="Response text"
							placeholder="Hello from Sample."
							value={scaffold.response_message}
							onChange={(event) =>
								onFieldChange("response_message", event.currentTarget.value)
							}
						/>
					</Stack>
				</Card>
				<Card className="panel-card" withBorder={true}>
					<Stack gap="sm">
						<Text fw={700}>Summary</Text>
						<Text size="sm">
							Target path: <Code>{`plugins/${pluginID}`}</Code>
						</Text>
						<Text size="sm">
							Active bundle: <Code>{bundleDir}</Code>
						</Text>
						<Text size="sm">Files to create:</Text>
						<Stack gap={4}>
							{files.map((file) => (
								<Code key={file} block={true}>
									{file}
								</Code>
							))}
						</Stack>
						<Text size="sm">
							Signing:{" "}
							<Code>{signingConfigured ? "available" : "not configured"}</Code>
						</Text>
					</Stack>
				</Card>
			</SimpleGrid>

			<SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
				<Card className="panel-card" withBorder={true}>
					<Stack gap="sm">
						<Text fw={700}>Storage</Text>
						{Object.entries(permissions.storage).map(([key, value]) => (
							<Checkbox
								key={key}
								label={key}
								checked={value}
								onChange={(event) =>
									onPermissionToggle(
										"storage",
										key,
										event.currentTarget.checked,
									)
								}
							/>
						))}
						<Text fw={700} mt="sm">
							Network
						</Text>
						{Object.entries(permissions.network).map(([key, value]) => (
							<Checkbox
								key={key}
								label={key}
								checked={value}
								onChange={(event) =>
									onPermissionToggle(
										"network",
										key,
										event.currentTarget.checked,
									)
								}
							/>
						))}
					</Stack>
				</Card>
				<Card className="panel-card" withBorder={true}>
					<Stack gap="sm">
						<Text fw={700}>Discord</Text>
						{Object.entries(permissions.discord).map(([key, value]) => (
							<Checkbox
								key={key}
								label={key}
								checked={value}
								onChange={(event) =>
									onPermissionToggle(
										"discord",
										key,
										event.currentTarget.checked,
									)
								}
							/>
						))}
						<Text fw={700} mt="sm">
							Automation
						</Text>
						<Checkbox
							label="jobs"
							checked={permissions.automation.jobs}
							onChange={(event) => onJobsToggle(event.currentTarget.checked)}
						/>
						{Object.entries(permissions.automation.events).map(
							([key, value]) => (
								<Checkbox
									key={key}
									label={key}
									checked={value}
									onChange={(event) =>
										onPermissionToggle(
											"automation",
											key,
											event.currentTarget.checked,
										)
									}
								/>
							),
						)}
					</Stack>
				</Card>
			</SimpleGrid>

			<Card className="panel-card" withBorder={true}>
				<Group justify="space-between">
					<Checkbox
						label="Sign the plugin after creation"
						checked={scaffold.sign}
						disabled={!signingConfigured}
						onChange={(event) =>
							onFieldChange("sign", event.currentTarget.checked)
						}
					/>
					<Button loading={busy === "plugins:scaffold"} onClick={onSubmit}>
						Create plugin
					</Button>
				</Group>
			</Card>
		</Stack>
	);
}
