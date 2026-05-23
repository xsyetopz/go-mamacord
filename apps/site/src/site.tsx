import { AppFrame, SectionCard, SectionTitle } from "@mamacord/web-ui";
import {
	Badge,
	Button,
	Code,
	Group,
	List,
	SimpleGrid,
	Stack,
	Text,
	ThemeIcon,
} from "@mantine/core";
import {
	IconBook2,
	IconBrandTwitch,
	IconCloudStorm,
	IconCodeDots,
	IconCompass,
	IconPlugConnected,
} from "@tabler/icons-react";

const pluginAPIs = [
	"`bot.ui`, `bot.effects`, `bot.runtime` for interaction and runtime helpers",
	"`bot.discord.*` for Discord reads and management behind permission gates",
	"`bot.store`, `bot.usersettings`, `bot.checkins`, `bot.reminders`, `bot.warnings`, `bot.audit` for persistent data APIs",
	"`bot.http.get` and `bot.http.get_json` for approved HTTPS fetches",
];

const guides = [
	"Plugin manifest + permissions",
	"Signing and trusted vendors",
	"Marketplace source layout",
	"Dashboard deployment and API origin wiring",
];

export function SiteApp() {
	return (
		<AppFrame>
			<Stack gap="xl">
				<SectionCard className="site-hero">
					<Stack gap="xl">
						<Badge variant="light" color="brand" radius="sm" w="fit-content">
							Public site
						</Badge>
						<Stack gap="md">
							<Text className="site-hero-title" fw={800}>
								One bot runtime. Separate public site. Separate admin dashboard.
							</Text>
							<Text size="lg" c="dimmed" maw={760}>
								mamacord keeps the landing page, docs, examples, and plugin API
								reference public while the admin dashboard stays isolated behind
								Discord auth.
							</Text>
						</Stack>
						<Group gap="sm">
							<Button
								component="a"
								href="https://github.com/xsyetopz/go-mamacord"
							>
								Open Repository
							</Button>
							<Button variant="default" component="a" href="#docs">
								Read docs
							</Button>
						</Group>
					</Stack>
				</SectionCard>

				<SimpleGrid cols={{ base: 1, md: 3 }} spacing="md">
					<SectionCard className="site-grid-card">
						<Stack gap="sm">
							<ThemeIcon size={42} variant="light" color="brand">
								<IconPlugConnected size={20} />
							</ThemeIcon>
							<Text fw={700}>Plugin platform</Text>
							<Text size="sm" c="dimmed">
								Bundled plugins and installed plugins share one runtime model:
								plugins are removable, signable, and discoverable through
								git-based sources.
							</Text>
						</Stack>
					</SectionCard>
					<SectionCard className="site-grid-card">
						<Stack gap="sm">
							<ThemeIcon size={42} variant="light" color="brand">
								<IconBrandTwitch size={20} />
							</ThemeIcon>
							<Text fw={700}>Twitch integration</Text>
							<Text size="sm" c="dimmed">
								Public stream lookups, account linking, and guild
								live-announcement flows are designed as first-class
								backend-backed capabilities.
							</Text>
						</Stack>
					</SectionCard>
					<SectionCard className="site-grid-card">
						<Stack gap="sm">
							<ThemeIcon size={42} variant="light" color="brand">
								<IconCloudStorm size={20} />
							</ThemeIcon>
							<Text fw={700}>Weather providers</Text>
							<Text size="sm" c="dimmed">
								WeatherKit is the initial provider behind a normalized weather
								interface so future providers can slot in without changing
								plugin code.
							</Text>
						</Stack>
					</SectionCard>
				</SimpleGrid>

				<section id="docs">
					<Stack gap="lg">
						<SectionTitle
							eyebrow="Docs"
							title="Public plugin API reference"
							description="The public site exposes the stable Lua host surface, configuration schemas, and deployment guidance without mixing it into the authenticated dashboard."
						/>
						<SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
							<SectionCard>
								<Stack gap="sm">
									<Group gap="xs">
										<ThemeIcon size={34} variant="light" color="brand">
											<IconCodeDots size={18} />
										</ThemeIcon>
										<Text fw={700}>Core Lua APIs</Text>
									</Group>
									<List size="sm" spacing="xs">
										{pluginAPIs.map((item) => (
											<List.Item key={item}>
												<Code>{item}</Code>
											</List.Item>
										))}
									</List>
								</Stack>
							</SectionCard>
							<SectionCard>
								<Stack gap="sm">
									<Group gap="xs">
										<ThemeIcon size={34} variant="light" color="brand">
											<IconBook2 size={18} />
										</ThemeIcon>
										<Text fw={700}>Guides and examples</Text>
									</Group>
									<List size="sm" spacing="xs">
										{guides.map((item) => (
											<List.Item key={item}>{item}</List.Item>
										))}
									</List>
									<Text size="sm" c="dimmed">
										Deep reference still lives in repo docs like{" "}
										<Code>docs/reference.md</Code>, while this site acts as the
										stable public entrypoint.
									</Text>
								</Stack>
							</SectionCard>
						</SimpleGrid>
					</Stack>
				</section>

				<SectionCard>
					<Stack gap="md">
						<SectionTitle
							eyebrow="Deployment"
							title="GitHub Pages without a custom domain"
							description="The public site is designed to build as a static app under the repository path, while the admin dashboard can stay self-hosted or be deployed separately against its API origin."
						/>
						<Group gap="sm">
							<ThemeIcon size={34} variant="light" color="brand">
								<IconCompass size={18} />
							</ThemeIcon>
							<Text size="sm" c="dimmed">
								Static public site on Pages. Authenticated dashboard on its own
								origin. Shared visual system underneath both.
							</Text>
						</Group>
					</Stack>
				</SectionCard>
			</Stack>
		</AppFrame>
	);
}
