import { Card, SimpleGrid, Stack, Table, Text } from "@mantine/core";
import { MetricCard } from "../components/MetricCard";
import { PageHeader } from "../components/PageHeader";
import type { MigrationStatus } from "../types";

type Props = {
	migrationStatus: MigrationStatus | null;
};

export function MigrationsPage({ migrationStatus }: Props) {
	return (
		<Stack gap="lg">
			<PageHeader title="Migrations" subtitle="Current version and pending migration files." />
			<SimpleGrid cols={{ base: 1, md: 3 }}>
				<MetricCard
					label="Current version"
					value={String(migrationStatus?.current_version ?? "—")}
				/>
				<MetricCard
					label="Applied"
					value={String(migrationStatus?.applied.length ?? 0)}
				/>
				<MetricCard
					label="Pending"
					value={String(migrationStatus?.pending.length ?? 0)}
				/>
			</SimpleGrid>
			<Card className="panel-card" withBorder={true}>
				<Stack gap="md">
					<Text fw={700}>Pending migrations</Text>
					<Table
						className="compact-table"
						striped={true}
						highlightOnHover={true}
					>
						<Table.Thead>
							<Table.Tr>
								<Table.Th>Version</Table.Th>
								<Table.Th>Name</Table.Th>
								<Table.Th>Kind</Table.Th>
							</Table.Tr>
						</Table.Thead>
						<Table.Tbody>
							{(migrationStatus?.pending ?? []).map((item) => (
								<Table.Tr key={`${item.version}-${item.name}`}>
									<Table.Td>{item.version}</Table.Td>
									<Table.Td>{item.name}</Table.Td>
									<Table.Td>{item.kind}</Table.Td>
								</Table.Tr>
							))}
							{(migrationStatus?.pending.length ?? 0) === 0 ? (
								<Table.Tr>
									<Table.Td colSpan={3}>
										<Text size="sm" c="dimmed">
											No pending migrations.
										</Text>
									</Table.Td>
								</Table.Tr>
							) : null}
						</Table.Tbody>
					</Table>
				</Stack>
			</Card>
		</Stack>
	);
}
