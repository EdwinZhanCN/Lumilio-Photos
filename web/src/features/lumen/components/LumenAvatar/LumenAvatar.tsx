import React, { useEffect, useRef } from "react";
import "@/styles/pyramid.css";

interface LumenAvatarProps {
  /** When true, the pyramid will spin. */
  start?: boolean;
  /** Size in rem units. Default is 1 (20rem base size). */
  size?: number;
  /** Additional CSS class name */
  className?: string;
  /** Custom style overrides */
  style?: React.CSSProperties;
}

/** 平滑重置函数：锁定当前位置，然后用 transition 回滚到初始状态 */
function smoothReset(
  el: HTMLElement,
  duration: number = 2000,
  easing: string = "ease-out",
) {
  // 1. 拿到当前 transform 矩阵
  const computed = getComputedStyle(el).transform;

  // 2. 如果没有变换或者是默认值，直接返回
  if (
    !computed ||
    computed === "none" ||
    computed === "matrix(1, 0, 0, 1, 0, 0)"
  ) {
    return;
  }

  // 3. 锁定当前位置
  el.style.transform = computed;
  // 4. 取消动画和类名
  el.style.animation = "none";
  el.style.webkitAnimation = "none";
  // 5. 强制回流
  void el.offsetHeight;
  // 6. 添加 transition
  el.style.transition = `transform ${duration}ms ${easing}`;
  el.style.webkitTransition = `transform ${duration}ms ${easing}`;
  // 7. 回滚到初始 transform（即没有旋转的状态）
  el.style.transform = "";
  // 8. transition 结束后清理内联样式
  const clean = () => {
    el.style.transition = "";
    el.style.webkitTransition = "";
    el.style.transform = "";
    el.style.animation = "";
    el.style.webkitAnimation = "";
    // 取消监听
    el.removeEventListener("transitionend", clean);
    el.removeEventListener("webkitTransitionEnd", clean);
  };
  el.addEventListener("transitionend", clean);
  el.addEventListener("webkitTransitionEnd", clean);
}

export const LumenAvatar: React.FC<LumenAvatarProps> = ({
  start = false,
  size = 1,
  className = "",
  style = {},
}) => {
  const axisRef = useRef<HTMLDivElement>(null);

  // 监听 start 变化，执行平滑开始/停止
  useEffect(() => {
    const el = axisRef.current;
    if (!el) return;

    if (start) {
      // → true：清理内联样式，恢复 spinning
      el.style.transition = "";
      el.style.webkitTransition = "";
      el.style.animation = "";
      el.style.webkitAnimation = "";
      el.style.transform = "";
      el.classList.add("spinning");
    } else {
      // → false：移除 spinning，再平滑回滚
      el.classList.remove("spinning");
      // 使用 requestAnimationFrame 确保类名移除后再执行平滑重置
      requestAnimationFrame(() => {
        smoothReset(el);
      });
    }
  }, [start]);

  const containerStyle: React.CSSProperties = {
    fontSize: `${size}rem`,
    ...style,
  };

  return (
    <div className={`lumen-avatar ${className}`.trim()} style={containerStyle}>
      <div className="pyramid">
        <div className="pyramid-gyro">
          <div
            ref={axisRef}
            className={`pyramid-axis${start ? " spinning" : ""}`}
          >
            <div className="pyramid-wall front" />
            <div className="pyramid-wall back" />
            <div className="pyramid-wall left" />
            <div className="pyramid-wall right" />
            <div className="bottom" />
            <div className="shadow" />
          </div>
        </div>
      </div>
    </div>
  );
};
