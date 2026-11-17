import { Monitor, Moon, Sun } from "lucide-react";
import { useEffect, useState } from "react";

type ThemeMode = "light" | "dark" | "auto";

function getInitialMode(): ThemeMode {
	if (typeof window === "undefined") return "auto";
	const stored = window.localStorage.getItem("theme");
	return stored === "light" || stored === "dark" || stored === "auto"
		? stored
		: "auto";
}

function applyThemeMode(mode: ThemeMode) {
	const prefersDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
	const resolved = mode === "auto" ? (prefersDark ? "dark" : "light") : mode;
	const root = document.documentElement;
	root.classList.remove("light", "dark");
	root.classList.add(resolved);
	if (mode === "auto") root.removeAttribute("data-theme");
	else root.setAttribute("data-theme", mode);
	root.style.colorScheme = resolved;
}

const NEXT: Record<ThemeMode, ThemeMode> = {
	light: "dark",
	dark: "auto",
	auto: "light",
};

export default function ThemeToggle() {
	const [mode, setMode] = useState<ThemeMode>("auto");

	useEffect(() => {
		const initial = getInitialMode();
		setMode(initial);
		applyThemeMode(initial);
	}, []);

	useEffect(() => {
		if (mode !== "auto") return;
		const media = window.matchMedia("(prefers-color-scheme: dark)");
		const onChange = () => applyThemeMode("auto");
		media.addEventListener("change", onChange);
		return () => media.removeEventListener("change", onChange);
	}, [mode]);

	function toggleMode() {
		const next = NEXT[mode];
		setMode(next);
		applyThemeMode(next);
		window.localStorage.setItem("theme", next);
	}

	const Icon = mode === "light" ? Sun : mode === "dark" ? Moon : Monitor;
	const label = `Theme: ${mode === "auto" ? "system" : mode}. Switch to ${NEXT[mode] === "auto" ? "system" : NEXT[mode]}.`;

	return (
		<button
			type="button"
			onClick={toggleMode}
			aria-label={label}
			title={label}
			className="grid size-9 place-items-center rounded-full border border-border bg-card text-muted-foreground transition hover:text-foreground"
		>
			<Icon className="size-4" />
		</button>
	);
}
