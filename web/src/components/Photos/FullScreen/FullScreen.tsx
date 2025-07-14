
import FullScreenCarousel from "./FullScreenCarousel/FullScreenCarousel";
import FullScreenToolbar from "./FullScreenToolbar/FullScreenToolbar";
import FullScreenInfo from "./FullScreenInfo/FullScreenInfo";
import { useState } from "react";

const FullScreen = () => {
  const [showInfo, setShowInfo] = useState(false);

  const toggleInfo = () => {
    setShowInfo(!showInfo);
  };

  return (
    <div>
      <FullScreenToolbar />
      <FullScreenCarousel />
      {showInfo && <FullScreenInfo />}
      <button onClick={toggleInfo}>Info</button>
    </div>
  );
};

export default FullScreen;
