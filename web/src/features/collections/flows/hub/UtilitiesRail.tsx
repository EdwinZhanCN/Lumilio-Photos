import { useNavigate } from "react-router-dom";
import Rail from "../../components/Rail";
import RailCard from "../../components/RailCard";
import { useUtilityShortcuts } from "../utilities/useUtilityShortcuts";

/**
 * UtilitiesRail surfaces fixed maintenance and smart utility shortcuts at the
 * top of the Collections page. The full list lives on the dedicated Utilities
 * page; this rail and that page share their cards via {@link useUtilityShortcuts}.
 */
export default function UtilitiesRail() {
  const navigate = useNavigate();
  const shortcuts = useUtilityShortcuts();

  return (
    <Rail>
      {shortcuts.map((shortcut) => (
        <RailCard
          key={shortcut.key}
          media={{ kind: "icon", icon: shortcut.icon, tone: shortcut.tone }}
          title={shortcut.title}
          onClick={() => navigate(shortcut.to)}
          className="w-48"
        />
      ))}
    </Rail>
  );
}
