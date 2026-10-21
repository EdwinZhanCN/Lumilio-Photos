import { Link, useLocation, type LinkProps } from "react-router-dom";

/** Carry the library URL through nested details without storing it globally. */
export default function MusicLink(props: LinkProps) {
  const location = useLocation();
  const inherited: unknown = location.state?.musicReturnTo;
  const returnTo =
    location.pathname === "/music"
      ? `${location.pathname}${location.search}`
      : typeof inherited === "string" && /^\/music(?:\?|$)/.test(inherited)
        ? inherited
        : undefined;
  const isBack =
    typeof props.to === "string" &&
    /^\/music(?:\?|$)/.test(props.to) &&
    location.pathname !== "/music";
  return (
    <Link
      {...props}
      to={isBack && returnTo ? returnTo : props.to}
      state={{ ...props.state, musicReturnTo: returnTo }}
    />
  );
}
