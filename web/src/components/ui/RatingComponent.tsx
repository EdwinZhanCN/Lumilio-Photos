import { useI18n } from "@/lib/i18n.tsx";

/**
 * **Rating Component**
 *
 * Interactive 5-star rating system with React 19 optimistic updates.
 *
 * This component works with parent components that use React 19's `useOptimistic`
 * hook to provide immediate UI feedback while maintaining data integrity.
 *
 * Flow:
 * 1. User clicks rating → Parent shows optimistic update immediately
 * 2. Parent calls API to persist change
 * 3. On success: optimistic update becomes real state
 * 4. On error: React automatically reverts to original database state
 *
 * @param rating - Current rating (optimistic or real database state)
 * @param onRatingChange - Callback to trigger optimistic update and API call
 */
interface RatingComponentProps {
  rating: number; // 0-5, where 0 means unrated (optimistic or database state)
  onRatingChange: (rating: number) => void;
  disabled?: boolean;
  size?: "sm" | "md" | "lg";
  showUnratedButton?: boolean;
  className?: string;
}

export default function RatingComponent({
  rating,
  onRatingChange,
  disabled = false,
  size = "md",
  showUnratedButton = true,
  className = "",
}: RatingComponentProps) {
  const { t } = useI18n();

  const getSizeClasses = () => {
    switch (size) {
      case "sm":
        return "rating-sm";
      case "lg":
        return "rating-lg";
      default:
        return "";
    }
  };

  const handleRatingClick = (newRating: number) => {
    if (disabled) return;
    // Trigger optimistic update and API call in parent component
    // React 19's useOptimistic provides immediate feedback with automatic revert on error
    onRatingChange(newRating);
  };

  return (
    <div className={`flex items-center gap-2 ${className}`}>
      {showUnratedButton && (
        <button
          type="button"
          className={`btn btn-xs ${rating === 0 ? "btn-primary" : ""}`}
          onClick={() => handleRatingClick(0)}
          disabled={disabled}
          title={t("rating.unrated")}
          aria-label={t("rating.unrated")}
        >
          {t("rating.none")}
        </button>
      )}
      <div className={`rating ${getSizeClasses()}`}>
        {[1, 2, 3, 4, 5].map((star) => (
          <input
            key={star}
            type="radio"
            name={`rating-${Math.random()}`}
            className={`mask mask-star-2 bg-orange-400 cursor-pointer transition-opacity ${
              disabled ? "opacity-50 cursor-not-allowed" : "hover:opacity-80"
            }`}
            aria-label={t("rating.stars", { count: star })}
            checked={rating === star}
            onChange={() => handleRatingClick(star)}
            disabled={disabled}
            title={t("rating.stars", { count: star })}
          />
        ))}
      </div>
    </div>
  );
}
