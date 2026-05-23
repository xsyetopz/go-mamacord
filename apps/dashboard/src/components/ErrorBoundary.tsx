import {
	Button,
	Code,
	Container,
	Group,
	Stack,
	Text,
	Title,
} from "@mantine/core";
import type { ReactNode } from "react";
import { Component } from "react";

type Props = {
	children: ReactNode;
};

type State = {
	hasError: boolean;
	message: string;
	stack: string;
};

const DEV_DETAILS_KEY = "mamacord-dev-details";

function shouldShowDiagnostics(): boolean {
	if (import.meta.env["DEV"]) {
		return true;
	}
	try {
		return window.localStorage.getItem(DEV_DETAILS_KEY) === "1";
	} catch {
		return false;
	}
}

export class ErrorBoundary extends Component<Props, State> {
	override state: State = { hasError: false, message: "", stack: "" };

	static getDerivedStateFromError(error: unknown): State {
		const message = error instanceof Error ? error.message : "Unknown error";
		const stack = error instanceof Error ? (error.stack ?? "") : "";
		return { hasError: true, message, stack };
	}

	override render() {
		if (!this.state.hasError) {
			return this.props.children;
		}

		const showDetails = shouldShowDiagnostics();
		return (
			<Container size="sm" py="xl">
				<Stack gap="md">
					<Title order={2}>Dashboard crashed unexpectedly</Title>
					<Text c="dimmed" size="sm">
						This is a bug. You can usually recover by reloading.
					</Text>
					<Group>
						<Button
							onClick={() => {
								window.location.reload();
							}}
						>
							Reload
						</Button>
						<Button
							variant="default"
							onClick={() => {
								window.location.hash = "#/setup";
							}}
						>
							Go to Setup
						</Button>
					</Group>
					{showDetails ? (
						<Stack gap="xs">
							<Text size="sm" fw={600}>
								Error details (dev)
							</Text>
							<Code block={true}>
								{this.state.message}
								{this.state.stack ? `\n\n${this.state.stack}` : ""}
							</Code>
						</Stack>
					) : null}
				</Stack>
			</Container>
		);
	}
}
