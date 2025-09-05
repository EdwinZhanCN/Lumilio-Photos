import React from "react";

export type AICategory = {
  id?: string;
  name: string;
  count?: number;
  imageUrl?: string;
  subtitle?: string;
};

export type AICategoryCarouselProps = {
  /**
   * Categories to display. If omitted, a default list is shown.
   */
  categories?: AICategory[];
  /**
   * Compute the display count for a category when `count` is not provided.
   * Receives the category and its index. Default: (index + 1) * 12
   */
  getCount?: (cat: AICategory, index: number) => number;
  /**
   * Callback when a category card is selected/clicked.
   */
  onSelect?: (cat: AICategory, index: number) => void;
  /**
   * Class applied to the outer carousel section
   */
  className?: string;
  /**
   * Class applied to each carousel item container
   */
  itemClassName?: string;
  /**
   * Class applied to the inner card
   */
  cardClassName?: string;
};

const DEFAULT_CATEGORIES: AICategory[] = [
  { name: "旅行记忆" },
  { name: "家庭时光" },
  { name: "自然景观" },
  { name: "美食记录" },
];

const AICategoryCarousel: React.FC<AICategoryCarouselProps> = ({
  categories = DEFAULT_CATEGORIES,
  getCount = (_cat, i) => (i + 1) * 12,
  onSelect,
  className = "",
  itemClassName = "",
  cardClassName = "",
}) => {
  return (
    <section
      className={`carousel carousel-center gap-4 p-4 bg-base-200 rounded-3xl ${className}`}
      aria-label="AI 分类展示"
    >
      {categories.map((cat, i) => {
        const key = cat.id ?? cat.name ?? `cat-${i}`;
        const count =
          typeof cat.count === "number" ? cat.count : getCount(cat, i);

        const handleSelect = () => onSelect?.(cat, i);

        return (
          <div key={key} className={`carousel-item ${itemClassName}`}>
            <div
              className={`card w-64 bg-base-100 shadow-xl hover:shadow-2xl transition-shadow cursor-pointer ${cardClassName}`}
              role="button"
              tabIndex={0}
              aria-label={`分类：${cat.name}，共 ${count} 个相关项目`}
              onClick={handleSelect}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  handleSelect();
                }
              }}
            >
              <figure className="aspect-video bg-base-200 overflow-hidden">
                {cat.imageUrl ? (
                  <img
                    src={cat.imageUrl}
                    alt={cat.name}
                    className="h-full w-full object-cover"
                    loading="lazy"
                  />
                ) : (
                  <div className="w-full h-full animate-pulse" />
                )}
              </figure>
              <div className="card-body">
                <h3 className="card-title">{cat.name}</h3>
                <p className="text-sm opacity-70">AI识别到{count}个相关项目</p>
                {cat.subtitle && (
                  <p className="text-xs text-base-content/60">{cat.subtitle}</p>
                )}
              </div>
            </div>
          </div>
        );
      })}
    </section>
  );
};

export default AICategoryCarousel;
