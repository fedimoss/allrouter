import React, { useRef, useState, useEffect } from 'react';
import { useIsMobile } from '../../hooks/common/useIsMobile';

const MarqueeLogos = ({ logos, speed = 1 }) => {
  const [isPaused, setIsPaused] = useState(false);
  const containerRef = useRef(null);
  const [containerWidth, setContainerWidth] = useState(0);
  const isMobile = useIsMobile();

  // 计算容器宽度
  useEffect(() => {
    const updateWidth = () => {
      if (containerRef.current) {
        setContainerWidth(containerRef.current.offsetWidth);
      }
    };
    updateWidth();
    window.addEventListener('resize', updateWidth);
    return () => window.removeEventListener('resize', updateWidth);
  }, []);

  // 复制一组logo用于无缝滚动
  const duplicatedLogos = [...logos, ...logos, ...logos];

  const baseDuration = 10; // speed=1时10s完成
  const duration = baseDuration / speed;

  return (
    <div
      ref={containerRef}
      className='landing-v2-logo-marquee'
      onMouseEnter={() => setIsPaused(true)}
      onMouseLeave={() => setIsPaused(false)}
    >
      <div
        className={`landing-v2-logo-marquee-content ${isPaused ? 'paused' : ''}`}
        style={{
          animationDuration: isMobile ? `${duration * 1.5}s` : `${duration}s`,
          animationTimingFunction: 'linear',
          animationIterationCount: 'infinite',
        }}
      >
        {duplicatedLogos.map((logo, index) => (
          <div key={`${logo.alt}-${index}`} className='landing-v2-logo-item'>
            <img
              src={logo.src}
              alt={logo.alt}
              className='landing-v2-partner-logo'
            />
            <span className='landing-v2-logo-name'>{logo.alt}</span>
          </div>
        ))}
      </div>
    </div>
  );
};

export default MarqueeLogos;