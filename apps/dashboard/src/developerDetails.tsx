import {
	createContext,
	type Dispatch,
	type ReactNode,
	type SetStateAction,
	useCallback,
	useContext,
	useEffect,
	useMemo,
	useState,
} from "react";

const STORAGE_KEY = "mamacord-dev-details";

type DeveloperDetailsContextValue = {
	enabled: boolean;
	setEnabled: Dispatch<SetStateAction<boolean>>;
	toggle: () => void;
};

const DeveloperDetailsContext =
	createContext<DeveloperDetailsContextValue | null>(null);

function readInitial(): boolean {
	try {
		return window.localStorage.getItem(STORAGE_KEY) === "1";
	} catch {
		return false;
	}
}

export function DeveloperDetailsProvider({
	children,
}: {
	children: ReactNode;
}) {
	const [enabled, setEnabled] = useState(readInitial);

	useEffect(() => {
		try {
			window.localStorage.setItem(STORAGE_KEY, enabled ? "1" : "0");
		} catch {
			// ignore
		}
	}, [enabled]);

	const toggle = useCallback(() => setEnabled((v) => !v), []);

	const value = useMemo(
		() => ({ enabled, setEnabled, toggle }),
		[enabled, toggle],
	);
	return (
		<DeveloperDetailsContext.Provider value={value}>
			{children}
		</DeveloperDetailsContext.Provider>
	);
}

export function useDeveloperDetails(): DeveloperDetailsContextValue {
	const value = useContext(DeveloperDetailsContext);
	if (!value) {
		throw new Error(
			"useDeveloperDetails must be used within DeveloperDetailsProvider",
		);
	}
	return value;
}
