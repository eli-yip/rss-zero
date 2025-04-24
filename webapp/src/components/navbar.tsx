// Navbar.tsx
import { Link } from "@heroui/react";
import {
  Avatar,
  Dropdown,
  DropdownItem,
  DropdownMenu,
  DropdownTrigger,
  Navbar as HeroUINavbar,
  NavbarBrand,
  NavbarContent,
  NavbarItem,
  NavbarMenu,
  NavbarMenuItem,
  NavbarMenuToggle,
} from "@heroui/react";
import { link as linkStyles } from "@heroui/react";
import clsx from "clsx";
import { useLocation } from "react-router-dom";

import { Logo } from "@/components/icons";
import { ThemeSwitch } from "@/components/theme-switch";
import { siteConfig } from "@/config/site";
import { useUserInfo } from "@/hooks/useUserInfo";

export const Navbar = () => {
  const pathname = useLocation().pathname;
  const { userInfo, isLoading, isError, logout } = useUserInfo();

  return (
    <HeroUINavbar shouldHideOnScroll maxWidth="xl" position="sticky">
      <NavbarContent className="basis-1/5 sm:basis-full" justify="start">
        <NavbarBrand className="max-w-fit gap-3">
          <Link
            className="flex items-center justify-start gap-1"
            color="foreground"
            href="/"
          >
            <Logo />
            <p className="font-bold text-inherit">M-Lib</p>
          </Link>
        </NavbarBrand>
        <div className="ml-2 hidden justify-start gap-4 sm:flex">
          {siteConfig.navItems.map((item) => (
            <NavbarItem key={item.href} isActive={pathname === item.href}>
              <Link
                className={clsx(
                  linkStyles({ color: "foreground" }),
                  "data-[active=true]:font-medium data-[active=true]:text-primary",
                )}
                color="foreground"
                href={item.href}
              >
                {item.label}
              </Link>
            </NavbarItem>
          ))}
        </div>
      </NavbarContent>

      <NavbarContent
        className="hidden basis-1/5 sm:flex sm:basis-full"
        justify="end"
      >
        <NavbarItem>
          <ThemeSwitch />
        </NavbarItem>

        <NavbarItem>
          {!isLoading && !isError && userInfo && (
            <Dropdown placement="bottom-end">
              <DropdownTrigger>
                <Avatar
                  as="button"
                  className="transition-transform"
                  src="https://i.pravatar.cc/150"
                  name={userInfo.username}
                  size="sm"
                />
              </DropdownTrigger>
              <DropdownMenu aria-label="用户菜单">
                <DropdownItem
                  key="username"
                  isReadOnly
                  showDivider
                  classNames={{
                    base: "data-[hover=true]:bg-transparent cursor-auto",
                  }}
                >
                  <p>您好，{userInfo.username}</p>
                </DropdownItem>
                <DropdownItem key="logout" color="danger" onPress={logout}>
                  退出登录
                </DropdownItem>
              </DropdownMenu>
            </Dropdown>
          )}
        </NavbarItem>
      </NavbarContent>

      <NavbarContent className="basis-1 pl-4 sm:hidden" justify="end">
        <ThemeSwitch />

        {!isLoading && !isError && userInfo && (
          <Dropdown placement="bottom-end">
            <DropdownTrigger>
              <Avatar
                as="button"
                className="transition-transform"
                src="https://i.pravatar.cc/150"
                name={userInfo.username}
                size="sm"
              />
            </DropdownTrigger>
            <DropdownMenu aria-label="用户菜单">
              <DropdownItem
                key="username"
                isReadOnly
                showDivider
                classNames={{
                  base: "data-[hover=true]:bg-transparent cursor-auto",
                }}
              >
                <p>您好，{userInfo.username}</p>
              </DropdownItem>
              <DropdownItem key="logout" color="danger" onPress={logout}>
                退出登录
              </DropdownItem>
            </DropdownMenu>
          </Dropdown>
        )}
        <NavbarMenuToggle />
      </NavbarContent>

      <NavbarMenu>
        <div className="mx-4 mt-2 flex flex-col gap-2">
          {siteConfig.navItems.map((item) => (
            <NavbarMenuItem
              key={`${item.href}`}
              isActive={pathname === item.href}
            >
              <Link
                className={clsx(
                  linkStyles({ color: "foreground" }),
                  "data-[active=true]:font-medium data-[active=true]:text-primary",
                )}
                color="foreground"
                href={item.href}
                size="lg"
              >
                {item.label}
              </Link>
            </NavbarMenuItem>
          ))}
        </div>
      </NavbarMenu>
    </HeroUINavbar>
  );
};
