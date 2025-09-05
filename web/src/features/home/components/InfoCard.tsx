import React from "react";
import { GlobeAltIcon } from "@heroicons/react/24/outline";
import { ExternalLink } from "lucide-react";

export type InfoLink = {
  label: string;
  href: string;
  icon?: React.ReactNode;
  external?: boolean;
  description?: string;
  badge?: string;
};

export type InfoCardProps = {
  /**
   * 标题文本
   * 默认："摄影网站资讯"
   */
  title?: string;
  /**
   * 副标题/描述
   * 默认："发现500px、Flickr等平台的优秀作品与摄影技巧分享"
   */
  description?: string;
  /**
   * 标题左侧图标，默认 GlobeAltIcon
   */
  icon?: React.ReactNode;
  /**
   * 链接列表（如 500px、Flickr、Unsplash 等）
   */
  links?: InfoLink[];
  /**
   * 自定义操作区域（放置在右下角）
   */
  actions?: React.ReactNode;
  /**
   * 点击默认“浏览资讯”按钮时触发
   */
  onBrowse?: () => void;
  /**
   * 默认按钮的文案
   * 默认："浏览资讯"
   */
  browseLabel?: string;
  /**
   * 卡片风格
   * - primary: 背景主色 + 反色文字（默认）
   * - neutral: 浅色背景
   * - accent: 强调色背景
   */
  variant?: "primary" | "neutral" | "accent";
  /**
   * 外层自定义类
   */
  className?: string;
};

const DEFAULT_LINKS: InfoLink[] = [
  {
    label: "500px",
    href: "https://500px.com/",
    external: true,
    description: "高质量摄影作品分享社区",
  },
  {
    label: "Flickr",
    href: "https://www.flickr.com/",
    external: true,
    description: "历史悠久的摄影作品平台",
  },
  {
    label: "Unsplash",
    href: "https://unsplash.com/",
    external: true,
    description: "免费高质量图片社区",
  },
  {
    label: "PetaPixel",
    href: "https://petapixel.com/",
    external: true,
    description: "摄影新闻与技巧",
  },
];

const variantClasses: Record<NonNullable<InfoCardProps["variant"]>, string> = {
  primary: "bg-primary text-primary-content",
  neutral: "bg-base-100 text-base-content",
  accent: "bg-accent text-accent-content",
};

const InfoCard: React.FC<InfoCardProps> = ({
  title = "摄影网站资讯",
  description = "发现500px、Flickr等平台的优秀作品与摄影技巧分享",
  icon,
  links = DEFAULT_LINKS,
  actions,
  onBrowse,
  browseLabel = "浏览资讯",
  variant = "primary",
  className = "",
}) => {
  const headerId = React.useId();

  return (
    <section
      className={`card ${variantClasses[variant]} shadow-xl ${className}`}
      role="region"
      aria-labelledby={headerId}
    >
      <div className="card-body">
        <h2 id={headerId} className="card-title">
          <span className="inline-flex items-center gap-2">
            <span className="text-current">
              {icon ?? <GlobeAltIcon className="size-6" />}
            </span>
            <span>{title}</span>
          </span>
        </h2>

        {description && (
          <p className="opacity-90 text-sm md:text-base">{description}</p>
        )}

        {links && links.length > 0 && (
          <ul className="mt-2 grid gap-2 sm:grid-cols-2">
            {links.map((link, i) => {
              const {
                label,
                href,
                icon: linkIcon,
                external = true,
                description: linkDesc,
                badge,
              } = link;

              return (
                <li
                  key={`${label}-${href}-${i}`}
                  className="flex items-start gap-3 p-3 rounded-xl bg-black/10 hover:bg-black/15 transition-colors"
                >
                  <div className="mt-0.5">
                    {linkIcon ?? (
                      <ExternalLink
                        className="size-4 opacity-80"
                        aria-hidden="true"
                      />
                    )}
                  </div>
                  <div className="min-w-0">
                    <a
                      className="font-medium underline-offset-2 hover:underline break-all"
                      href={href}
                      target={external ? "_blank" : undefined}
                      rel={external ? "noopener noreferrer" : undefined}
                    >
                      {label}
                    </a>
                    {badge && (
                      <span className="badge badge-sm ml-2 align-middle">
                        {badge}
                      </span>
                    )}
                    {linkDesc && (
                      <div className="text-xs opacity-80 mt-0.5">
                        {linkDesc}
                      </div>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
        )}

        <div className="card-actions justify-end mt-2">
          {actions ? (
            actions
          ) : onBrowse ? (
            <button type="button" className="btn btn-secondary" onClick={onBrowse}>
              {browseLabel}
            </button>
          ) : (
            // Fallback: If no onBrowse and no custom actions, guide the user
            <a
              href={links?.[0]?.href ?? "#"}
              target="_blank"
              rel="noopener noreferrer"
              className="btn btn-secondary"
            >
              {browseLabel}
            </a>
          )}
        </div>
      </div>
    </section>
  );
};

export default InfoCard;
