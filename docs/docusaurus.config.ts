import { themes as prismThemes } from "prism-react-renderer";
import type { Config } from "@docusaurus/types";
import type * as Preset from "@docusaurus/preset-classic";

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

const config: Config = {
  title: "Cyphernetes",
  tagline: "A Kubernetes Query Language",
  favicon: "img/favicon.ico",

  headTags: [
    {
      tagName: 'link',
      attributes: {
        rel: 'preconnect',
        href: 'https://fonts.googleapis.com',
      },
    },
    {
      tagName: 'link',
      attributes: {
        rel: 'preconnect',
        href: 'https://fonts.gstatic.com',
        crossorigin: 'anonymous',
      },
    },
    {
      tagName: 'link',
      attributes: {
        rel: 'stylesheet',
        href: 'https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@300;400;500;600;700&display=swap',
      },
    },
    {
      tagName: 'style',
      attributes: {
        type: 'text/css',
      },
      innerHTML: `
        :root {
          --ifm-font-family-base: 'Space Grotesk', system-ui, sans-serif !important;
          --ifm-heading-font-family: 'Space Grotesk', system-ui, sans-serif !important;
        }
        html, body, #__docusaurus {
          font-family: 'Space Grotesk', system-ui, -apple-system, sans-serif !important;
        }
      `,
    },
  ],

  // Set the production url of your site here
  url: "https://docs.cyphernet.es",
  // Set the /<baseUrl>/ pathname under which your site is served
  // For GitHub pages deployment, it is often '/<projectName>/'
  baseUrl: "/",

  // GitHub pages deployment config.
  // If you aren't using GitHub pages, you don't need these.
  organizationName: "avitaltamir", // Usually your GitHub org/user name.
  projectName: "cyphernetes", // Usually your repo name.

  onBrokenLinks: "throw",
  onBrokenMarkdownLinks: "warn",

  // Even if you don't use internationalization, you can use this field to set
  // useful metadata like html lang. For example, if your site is Chinese, you
  // may want to replace "en" with "zh-Hans".
  i18n: {
    defaultLocale: "en",
    locales: ["en"],
  },

  presets: [
    [
      "classic",
      {
        docs: {
          sidebarPath: "./sidebars.ts",
          editUrl: "https://github.com/avitaltamir/cyphernetes/tree/main/docs/",
          breadcrumbs: false,
        },
        blog: false,
        theme: {
          customCss: "./src/css/custom.css",
        },
      } satisfies Preset.Options,
    ],
  ],

  plugins: [
    [
      "docusaurus-plugin-generate-llms-txt",
      {
        outputFile: "llms.txt",
      },
    ],
  ],

  themeConfig: {
    docs: {
      sidebar: {
        hideable: true,
      },
    },
    colorMode: {
      defaultMode: "dark",
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: "Cyphernetes",
      logo: {
        alt: "Cyphernetes Logo",
        src: "img/logo.png",
      },
      items: [
        {
          type: "docSidebar",
          sidebarId: "tutorialSidebar",
          position: "left",
          label: "Documentation",
        },
        {
          href: "https://github.com/avitaltamir/cyphernetes",
          position: "right",
          label: "GitHub",
          className: "header-github-link",
          "aria-label": "GitHub repository",
        },
      ],
    },
    footer: {
      style: "dark",
      links: [
        {
          title: "Documentation",
          items: [
            {
              label: "Getting Started",
              to: "/docs/installation",
            },
            {
              label: "Examples",
              to: "/docs/examples",
            },
          ],
        },
        {
          title: "Community",
          items: [
            {
              label: "GitHub",
              href: "https://github.com/avitaltamir/cyphernetes",
            },
            {
              label: "Discussions",
              href: "https://github.com/avitaltamir/cyphernetes/discussions",
            },
          ],
        },
        {
          title: "Social",
          items: [
            {
              label: "LinkedIn",
              href: "https://www.linkedin.com/company/cyphernetes",
            },
          ],
        },
        {
          title: "Contact",
          items: [
            {
              label: "team@cyphernet.es",
              href: "mailto:team@cyphernet.es",
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Cyphernetes`,
    },
    prism: {
      theme: prismThemes.vsDark,
      darkTheme: prismThemes.vsDark,
      defaultLanguage: "bash",
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
