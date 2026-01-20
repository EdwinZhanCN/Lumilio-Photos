import React, { useEffect, useRef, useState } from "react";
import "@/styles/pyramid.css";

interface LumilioAvatarProps {
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

export const LumilioAvatar: React.FC<LumilioAvatarProps> = ({
  start = false,
  size = 1,
  className = "",
  style = {},
}) => {
  const axisRef = useRef<HTMLDivElement>(null);
  const animationStartTimeRef = useRef<number | null>(null);
  const stopRequestedRef = useRef<boolean>(false);
  const [isSpinning, setIsSpinning] = useState(false);

  // Animation duration in milliseconds
  const ANIMATION_DURATION = 2000;

  // 计算剩余动画时间
  const getRemainingTime = (
    startTime: number,
    duration: number = ANIMATION_DURATION,
  ) => {
    const elapsed = Date.now() - startTime;
    return Math.max(0, duration - (elapsed % duration));
  };

  // 监听 start 变化，执行平滑开始/停止
  useEffect(() => {
    const el = axisRef.current;
    if (!el) return;

    if (start) {
      // 清除停止请求标志
      stopRequestedRef.current = false;

      // 如果当前没有在旋转，开始动画
      if (!isSpinning) {
        setIsSpinning(true);
        animationStartTimeRef.current = Date.now();
      }
    } else {
      // 设置停止请求标志，但不立即停止
      stopRequestedRef.current = true;

      // 只有在旋转时才需要处理停止逻辑
      if (isSpinning && animationStartTimeRef.current) {
        const remainingTime = getRemainingTime(animationStartTimeRef.current);

        // 设置定时器，在当前动画循环结束后停止
        const stopTimeout = setTimeout(() => {
          if (stopRequestedRef.current && el) {
            // 只有在没有重新启动的情况下才执行停止
            if (!start) {
              el.classList.remove("spinning");
              requestAnimationFrame(() => {
                smoothReset(el);
              });
              setIsSpinning(false);
              animationStartTimeRef.current = null;
            }
          }
        }, remainingTime);

        return () => clearTimeout(stopTimeout);
      }
    }
  }, [start, isSpinning]);

  // 更新 DOM 类名
  useEffect(() => {
    const el = axisRef.current;
    if (!el) return;

    if (isSpinning) {
      el.classList.add("spinning");
    } else {
      el.classList.remove("spinning");
    }
  }, [isSpinning]);

  const containerStyle: React.CSSProperties = {
    fontSize: `${size}rem`,
    ...style,
  };

  return (
    <div className={`lumen-avatar ${className}`.trim()} style={containerStyle}>
      <div className="pyramid">
        <div className="pyramid-gyro">
          <div ref={axisRef} className="pyramid-axis">
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
