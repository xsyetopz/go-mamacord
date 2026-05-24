import { Card, Code, SimpleGrid, Stack, Text } from "@mantine/core";
import { MetricCard } from "../components/MetricCard";
import { PageHeader } from "../components/PageHeader";
import { prettyDate } from "../format";
import type { StatusResponse } from "../types";

type Props = {
	status: StatusResponse;
};

export function OverviewPage({ status }: Props) {
	const cards = [
		{ label: "Ready", value: status.snapshot.ready ? "Yes" : "No" },
		{
			label: "Modules",
			value: `${status.snapshot.enabled_module_count} enabled / ${status.snapshot.module_count} total`,
		},
		{
			label: "Plugins",
			value: `${status.snapshot.enabled_plugin_count} enabled / ${status.snapshot.plugin_count} total`,
		},
		{
			label: "Commands",
			value: `${status.snapshot.slash_command_count} slash, ${status.snapshot.user_command_count} user, ${status.snapshot.message_command_count} message`,
		},
		{
			label: "Interactions",
			value: String(status.snapshot.interactions_total),
		},
		{
			label: "Failures",
			value: `${status.snapshot.interaction_failures} interaction, ${status.snapshot.plugin_failures} plugin, ${status.snapshot.automation_failures} automation`,
		},
	];

	return (
		<Stack gap="lg">
			<PageHeader
				title="Overview"
				subtitle="Current runtime state and build details."
			/>
			<SimpleGrid cols={{ base: 1, md: 3 }}>
				{cards.map((card) => (
					<MetricCard key={card.label} label={card.label} value={card.value} />
				))}
			</SimpleGrid>
			<SimpleGrid cols={{ base: 1, md: 2 }}>
				<Card className="panel-card" withBorder={true}>
					<Stack gap="sm">
						<Text fw={700}>Build</Text>
						<Text size="sm">
							Version: <Code>{status.build.version || "dev"}</Code>
						</Text>
						{status.build.repository ? (
							<Text size="sm">
								Repository: <Code>{status.build.repository}</Code>
							</Text>
						) : null}
						<Text size="sm">
							Started: <Code>{prettyDate(status.snapshot.started_at)}</Code>
						</Text>
						<Text size="sm">
							Mode:{" "}
							<Code>
								{status.config.prod_mode ? "production" : "development"}
							</Code>
						</Text>
					</Stack>
				</Card>
				<Card className="panel-card" withBorder={true}>
					<Stack gap="sm">
						<Text fw={700}>Paths</Text>
						<Text size="sm">
							Storage:{" "}
							<Code>
								{status.config.storage_backend}
								{status.config.storage_target
									? ` (${status.config.storage_target})`
									: ""}
							</Code>
						</Text>
						<Text size="sm">
							Bundled plugins: <Code>{status.config.bundled_plugins_dir}</Code>
							<br />
							User plugins: <Code>{status.config.user_plugins_dir}</Code>
						</Text>
						<Text size="sm">
							Migrations: <Code>{status.config.migrations_dir}</Code>
						</Text>
						<Text size="sm">
							Admin API: <Code>{status.config.admin_addr || "disabled"}</Code>
						</Text>
						<Text size="sm">
							Runtime roles:{" "}
							<Code>{status.config.runtime_roles.join(", ") || "none"}</Code>
						</Text>
					</Stack>
				</Card>
			</SimpleGrid>
		</Stack>
	);
}
