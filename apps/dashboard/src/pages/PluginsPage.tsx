import {
	Button,
	Card,
	Code,
	Group,
	SimpleGrid,
	Stack,
	Table,
	Text,
} from "@mantine/core";
import {
	IconFileCheck,
	IconFileX,
	IconPlus,
	IconRefresh,
	IconShieldCheck,
	IconShieldX,
} from "@tabler/icons-react";
import { CopyIconButton } from "../components/CopyIconButton";
import { PageHeader } from "../components/PageHeader";
import { BoolStatusIconBadge } from "../components/StatusIconBadge";
import { useDeveloperDetails } from "../developerDetails";
import type { PluginSummary } from "../types";

function pluginOriginLabel(plugin: PluginSummary): string {
	switch (plugin.provenance_kind) {
		case "bundled":
			return "Bundled plugin";
		case "marketplace":
			return "Marketplace plugin";
		default:
			return "User plugin";
	}
}

type Props = {
	plugins: PluginSummary[];
	busy: string | null;
	signingConfigured: boolean;
	onReload: () => void;
	onCreatePlugin: () => void;
	onSignPlugin: (pluginID: string) => void;
};

export function PluginsPage({
	plugins,
	busy,
	signingConfigured,
	onReload,
	onCreatePlugin,
	onSignPlugin,
}: Props) {
	const { enabled: devDetailsEnabled } = useDeveloperDetails();

	return (
		<Stack gap="md">
			<PageHeader
				title="Plugins"
				subtitle="Install state, trust state, and actions."
				primaryAction={
					<Button
						variant="default"
						leftSection={<IconPlus size={16} />}
						onClick={onCreatePlugin}
					>
						Create
					</Button>
				}
				secondaryActions={[
					{
						key: "reload",
						label: "Reload",
						icon: <IconRefresh size={16} />,
						onClick: onReload,
						loading: busy === "plugins:reload",
					},
				]}
			/>

			<Card className="panel-card" withBorder={true} visibleFrom="sm">
				<Table className="compact-table" striped={true} highlightOnHover={true}>
					<Table.Thead>
						<Table.Tr>
							<Table.Th>Plugin</Table.Th>
							<Table.Th>Version</Table.Th>
							<Table.Th>Commands</Table.Th>
							<Table.Th>Status</Table.Th>
							<Table.Th>Signature</Table.Th>
							<Table.Th>Actions</Table.Th>
						</Table.Tr>
					</Table.Thead>
					<Table.Tbody>
						{plugins.map((plugin) => (
							<Table.Tr key={plugin.id}>
								<Table.Td>
									<Stack gap={1}>
										<Text fw={600}>{plugin.name || plugin.id}</Text>
										<Text size="xs" c="dimmed">
											{pluginOriginLabel(plugin)}
										</Text>
										{devDetailsEnabled ? (
											<Stack gap={2}>
												<Group gap="xs">
													<Code>{plugin.id}</Code>
													<CopyIconButton
														value={plugin.id}
														label="Copy plugin ID"
													/>
												</Group>
												{plugin.git_revision ? (
													<Text size="xs" c="dimmed">
														Revision: <Code>{plugin.git_revision}</Code>
													</Text>
												) : null}
											</Stack>
										) : null}
									</Stack>
								</Table.Td>
								<Table.Td>{plugin.version || "—"}</Table.Td>
								<Table.Td>{plugin.commands.join(", ") || "—"}</Table.Td>
								<Table.Td>
									<BoolStatusIconBadge
										value={plugin.loaded}
										labelTrue="Loaded"
										labelFalse="Not loaded"
									/>
								</Table.Td>
								<Table.Td>
									<Group gap="xs">
										<BoolStatusIconBadge
											value={plugin.has_signature_file}
											labelTrue="Signature file present"
											labelFalse="Signature file missing"
											iconTrue={({ size }) => <IconFileCheck size={size} />}
											iconFalse={({ size }) => <IconFileX size={size} />}
										/>
										<BoolStatusIconBadge
											value={plugin.signed}
											labelTrue="Trusted (signed)"
											labelFalse="Unsigned"
											iconTrue={({ size }) => <IconShieldCheck size={size} />}
											iconFalse={({ size }) => <IconShieldX size={size} />}
										/>
									</Group>
								</Table.Td>
								<Table.Td>
									<Group gap="xs" className="table-actions">
										<Button
											size="xs"
											variant="outline"
											disabled={!signingConfigured}
											loading={busy === `plugin:sign:${plugin.id}`}
											onClick={() => onSignPlugin(plugin.id)}
										>
											Sign
										</Button>
									</Group>
								</Table.Td>
							</Table.Tr>
						))}
					</Table.Tbody>
				</Table>
			</Card>

			<SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md" hiddenFrom="sm">
				{plugins.map((plugin) => (
					<Card key={plugin.id} className="panel-card" withBorder={true}>
						<Stack gap="sm">
							<Group justify="space-between" align="flex-start">
								<Stack gap={2}>
									<Text fw={700}>{plugin.name || plugin.id}</Text>
									<Text size="xs" c="dimmed">
										{plugin.version || "No version"} · {plugin.commands.length}{" "}
										commands
									</Text>
									<Text size="xs" c="dimmed">
										{pluginOriginLabel(plugin)}
									</Text>
									{devDetailsEnabled ? (
										<Stack gap={2}>
											<Group gap="xs">
												<Code>{plugin.id}</Code>
												<CopyIconButton
													value={plugin.id}
													label="Copy plugin ID"
												/>
											</Group>
											{plugin.git_revision ? (
												<Text size="xs" c="dimmed">
													Revision: <Code>{plugin.git_revision}</Code>
												</Text>
											) : null}
										</Stack>
									) : null}
								</Stack>
								<BoolStatusIconBadge
									value={plugin.loaded}
									labelTrue="Loaded"
									labelFalse="Not loaded"
								/>
							</Group>
							<Group gap="xs">
								<BoolStatusIconBadge
									value={plugin.has_signature_file}
									labelTrue="Signature file present"
									labelFalse="Signature file missing"
									iconTrue={({ size }) => <IconFileCheck size={size} />}
									iconFalse={({ size }) => <IconFileX size={size} />}
									variant="light"
								/>
								<BoolStatusIconBadge
									value={plugin.signed}
									labelTrue="Trusted (signed)"
									labelFalse="Unsigned"
									iconTrue={({ size }) => <IconShieldCheck size={size} />}
									iconFalse={({ size }) => <IconShieldX size={size} />}
									variant="light"
								/>
							</Group>
							<Button
								variant="light"
								disabled={!signingConfigured}
								loading={busy === `plugin:sign:${plugin.id}`}
								onClick={() => onSignPlugin(plugin.id)}
							>
								Sign plugin
							</Button>
						</Stack>
					</Card>
				))}
			</SimpleGrid>
		</Stack>
	);
}
