import { defineConfig, godoc, markdown } from "sourcey";

export default defineConfig({
  name: "SecretSync",
  siteUrl: "https://jonbogaty.com",
  baseUrl: "/secrets-sync",
  // Static hosts do not generally provide extensionless-route fallbacks.
  prettyUrls: false,
  repo: "https://github.com/jbcom/secrets-sync",
  editBranch: "main",
  editBasePath: "docs",
  logo: {
    light: "./assets/secrets-sync-hero.png",
    dark: "./assets/secrets-sync-hero.png",
    href: "/secrets-sync/",
  },
  ogImage: "./assets/secrets-sync-hero.png",
  theme: {
    preset: "default",
    colors: {
      primary: "#0f766e",
      light: "#14b8a6",
      dark: "#115e59",
    },
    fonts: {
      sans: "Inter, ui-sans-serif, system-ui, sans-serif",
      mono: "SFMono-Regular, Consolas, Liberation Mono, monospace",
    },
    layout: {
      sidebar: "17rem",
      content: "52rem",
    },
    css: ["./brand.css"],
  },
  navigation: {
    tabs: [
      {
        tab: "Documentation",
        slug: "",
        source: markdown({
          groups: [
            {
              group: "Getting Started",
              pages: [
                "GETTING_STARTED",
                "getting-started/installation",
                "getting-started/quickstart",
                "USAGE",
                "FAQ",
              ],
            },
            {
              group: "Guides",
              pages: [
                "PIPELINE",
                "TWO_PHASE_ARCHITECTURE",
                "DEPLOYMENT",
                "GITHUB_ACTIONS",
                "ACTION_QUICK_REFERENCE",
                "OBSERVABILITY",
                "ERROR_CONTEXT",
                "PYTHON_BINDINGS",
              ],
            },
            {
              group: "Project",
              pages: [
                "ARCHITECTURE",
                "ARCHITECTURE_AUDIT",
                "OWNERSHIP",
                "SECURITY",
                "PRIVACY",
                "MARKETPLACE",
                "PUBLISHING_CHECKLIST",
                "ROADMAP",
                "SUPPORT",
              ],
            },
            {
              group: "Contributing",
              pages: [
                "development/contributing",
                "testing/organizations-discovery-integration-tests",
              ],
            },
          ],
        }),
      },
      {
        tab: "Go API",
        slug: "api",
        source: godoc({
          module: "..",
          packages: ["./pkg/...", "./python/secrets_sync"],
          mode: "live",
          includeTests: true,
          sourceBasePath: "",
        }),
      },
    ],
  },
  navbar: {
    links: [
      { label: "GitHub", href: "https://github.com/jbcom/secrets-sync" },
      { label: "Releases", href: "https://github.com/jbcom/secrets-sync/releases" },
    ],
  },
  footer: {
    links: [
      { label: "Security", href: "/secrets-sync/SECURITY.html" },
      { label: "MIT License", href: "https://github.com/jbcom/secrets-sync/blob/main/LICENSE" },
    ],
  },
  search: {
    featured: ["GETTING_STARTED", "PIPELINE", "SECURITY"],
  },
});
