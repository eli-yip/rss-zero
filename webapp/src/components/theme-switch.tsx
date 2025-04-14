import { type SwitchProps, useSwitch } from "@heroui/react";
import { VisuallyHidden } from "@react-aria/visually-hidden";
import clsx from "clsx";
import { type FC, useEffect, useState } from "react";

import { MoonFilledIcon, SunFilledIcon } from "@/components/icons";
import { useTheme } from "@/context/theme-context";

interface ThemeSwitchProps {
  className?: string;
  classNames?: SwitchProps["classNames"];
}

export const ThemeSwitch: FC<ThemeSwitchProps> = ({
  className,
  classNames,
}) => {
  const [isMounted, setIsMounted] = useState(false);

  const { state, dispatch } = useTheme();

  const onChange = () => {
    dispatch({ type: "TOGGLE" });
  };

  const {
    Component,
    slots,
    isSelected,
    getBaseProps,
    getInputProps,
    getWrapperProps,
  } = useSwitch({
    isSelected: state.theme === "light",
    onChange,
  });

  useEffect(() => {
    setIsMounted(true);
  }, []);

  // Prevent Hydration Mismatch
  if (!isMounted) return <div className="h-6 w-6" />;

  return (
    <Component
      aria-label={isSelected ? "Switch to dark mode" : "Switch to light mode"}
      {...getBaseProps({
        className: clsx(
          "px-px transition-opacity hover:opacity-80 cursor-pointer",
          className,
          classNames?.base,
        ),
      })}
    >
      <VisuallyHidden>
        <input {...getInputProps()} />
      </VisuallyHidden>
      <div
        {...getWrapperProps()}
        className={slots.wrapper({
          class: clsx(
            [
              "h-auto w-auto",
              "bg-transparent",
              "rounded-lg",
              "flex items-center justify-center",
              "group-data-[selected=true]:bg-transparent",
              "!text-default-500",
              "pt-px",
              "px-0",
              "mx-0",
            ],
            classNames?.wrapper,
          ),
        })}
      >
        {isSelected ? (
          <MoonFilledIcon size={22} />
        ) : (
          <SunFilledIcon size={22} />
        )}
      </div>
    </Component>
  );
};
