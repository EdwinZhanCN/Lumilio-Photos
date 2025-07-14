
import SortDropDown from "./SortDropDown";

const PhotosToolBar = () => {
  return (
    <div className="flex gap-2 items-center mb-4">
      <h1 className="text-2xl font-bold">Photos</h1>
      <SortDropDown />
    </div>
  );
};

export default PhotosToolBar;
