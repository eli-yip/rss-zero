export type SiteConfig = typeof siteConfig;

export const siteConfig = {
  name: "墨家第N基地",
  navItems: [
    {
      label: "Home",
      href: "/",
    },
    {
      label: "随便看看",
      href: "/random",
    },
    {
      label: "历史文章",
      href: "/archive",
    },
    {
      label: "Blog",
      href: "/blog",
    },
    {
      label: "About",
      href: "/about",
    },
  ],
  links: {
    github: "https://github.com/frontio-ai/heroui",
    twitter: "https://twitter.com/hero_ui",
    docs: "https://heroui.com",
    discord: "https://discord.gg/9b6yyZKmH4",
    sponsor: "https://patreon.com/jrgarciadev",
  },
};
