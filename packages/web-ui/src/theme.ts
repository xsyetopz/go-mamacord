import { createTheme, localStorageColorSchemeManager } from "@mantine/core";

const brandBlue = [
	"#edf4ff",
	"#d0e2ff",
	"#a6c8ff",
	"#78a9ff",
	"#4589ff",
	"#0f62fe",
	"#0043ce",
	"#002d9c",
	"#001d6c",
	"#001141",
] as const;

const wongSuccess = [
	"#e6faf3",
	"#c2f0e0",
	"#96e6cb",
	"#62dbb4",
	"#2ccf9c",
	"#009e73",
	"#00865f",
	"#006c4d",
	"#00523b",
	"#003727",
] as const;

const wongWarning = [
	"#fff5e5",
	"#ffe8c2",
	"#ffd799",
	"#ffc270",
	"#ffad47",
	"#e69f00",
	"#c98800",
	"#9f6d00",
	"#755000",
	"#4d3300",
] as const;

const wongDanger = [
	"#ffece2",
	"#ffd7c2",
	"#ffbd99",
	"#ffa06f",
	"#ff7f47",
	"#d55e00",
	"#b44f00",
	"#8f3f00",
	"#6a2f00",
	"#401c00",
] as const;

export const siteTheme = createTheme({
	colors: {
		brand: brandBlue,
		success: wongSuccess,
		warning: wongWarning,
		danger: wongDanger,
	},
	primaryColor: "brand",
	defaultRadius: "md",
	fontFamily:
		'"Iosevka Aile", "IBM Plex Sans", "Segoe UI", -apple-system, BlinkMacSystemFont, sans-serif',
	headings: {
		fontFamily:
			'"Iosevka Aile", "IBM Plex Sans", "Segoe UI", -apple-system, BlinkMacSystemFont, sans-serif',
		fontWeight: "700",
	},
	spacing: {
		xs: "0.5rem",
		sm: "0.75rem",
		md: "1rem",
		lg: "1.25rem",
		xl: "1.75rem",
	},
	components: {
		Text: {
			defaultProps: {
				component: "div",
			},
		},
		Button: {
			defaultProps: {
				radius: "md",
			},
		},
		Card: {
			defaultProps: {
				radius: "md",
			},
		},
		Table: {
			defaultProps: {
				verticalSpacing: "xs",
				horizontalSpacing: "sm",
			},
		},
	},
});

export const colorSchemeManager = localStorageColorSchemeManager({
	key: "mamacord-color-scheme",
});
