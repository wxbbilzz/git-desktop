import type { ButtonHTMLAttributes, ReactNode } from "react";

// 胶囊按钮：整个界面的统一交互元件。
// 所有按钮都走这里，避免样式在各地漂移。
type Variant = "default" | "primary" | "success" | "danger" | "ghost";

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: "md" | "sm";
  icon?: ReactNode;
  children?: ReactNode;
}

export function PillButton({
  variant = "default",
  size = "md",
  icon,
  children,
  className,
  ...rest
}: Props) {
  const classes = ["pill"];
  if (variant !== "default") classes.push(variant);
  if (size === "sm") classes.push("sm");
  if (icon && !children) classes.push("icon");
  if (className) classes.push(className);

  return (
    <button className={classes.join(" ")} {...rest}>
      {icon}
      {children}
    </button>
  );
}
