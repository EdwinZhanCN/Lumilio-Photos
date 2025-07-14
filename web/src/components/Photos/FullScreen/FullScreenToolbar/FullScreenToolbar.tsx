
const FullScreenToolbar = () => {
  // TODO: Implement FullScreenToolbar
  const handleLike = () => {
    // TODO: Add to liked photos
    console.log("Like action placeholder");
  };

  const handleDelete = () => {
    // TODO: Delete photo
    console.log("Delete action placeholder");
  };

  const handleAddToAlbum = () => {
    // TODO: Add to album
    console.log("Add to album placeholder");
  };

  return (
    <div>
      <button onClick={handleLike}>Like</button>
      <button onClick={handleDelete}>Delete</button>
      <button onClick={handleAddToAlbum}>Add to Album</button>
    </div>
  );
};

export default FullScreenToolbar;
