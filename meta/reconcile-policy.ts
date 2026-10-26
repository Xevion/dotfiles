// Settings apps rewrite on their own, per source file. Anything not listed here
// is a real setting and shows up in git or chezmoi diffs as usual.

export type Value = string | number | boolean;

export interface Policy {
  format: "jsonc" | "ini";
  /** "/"-separated key path -> the value recorded in the repo; null leaves the key out. */
  volatile: Record<string, Value | null>;
  /** Home-relative deployed path, for files chezmoi writes through a modify_ script. */
  target?: string;
}

export const POLICIES: Record<string, Policy> = {
  "home/.managed/zed/settings.linux.json": {
    format: "jsonc",
    volatile: { "theme/mode": "dark", "icon_theme/mode": "dark" },
  },
  "home/.managed/cline/global-settings.json": {
    format: "jsonc",
    volatile: { planActMode: "act" },
  },
  // OBS saves by rename, which replaces a symlink, so these deploy via modify_.
  "home/.managed/obs-studio/global.ini": {
    format: "ini",
    target: ".config/obs-studio/global.ini",
    volatile: {
      "General/InstallGUID": null,
      "General/InfoLastVersion": null,
      "General/LastVersion": null,
      "BasicWindow/geometry": null,
      "BasicWindow/DockState": null,
      "ScriptLogWindow/geometry": null,
      "PropertiesWindow/cx": null,
      "PropertiesWindow/cy": null,
    },
  },
  "home/.managed/obs-studio/basic/profiles/Untitled/basic.ini": {
    format: "ini",
    target: ".config/obs-studio/basic/profiles/Untitled/basic.ini",
    volatile: { "Panels/CookieId": null },
  },
};
