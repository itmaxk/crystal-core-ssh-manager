import React from 'react';

interface CrystalLogoProps {
  className?: string;
}

export const CrystalLogo: React.FC<CrystalLogoProps> = ({ className = 'h-5 w-5' }) => (
  <svg viewBox="0 0 64 64" fill="none" className={className} aria-label="Crystal-Core SSH Manager">
    <defs>
      <linearGradient id="ccGrad" x1="8" y1="4" x2="56" y2="60" gradientUnits="userSpaceOnUse">
        <stop offset="0%" stopColor="#38BDF8" />
        <stop offset="55%" stopColor="#0284C7" />
        <stop offset="100%" stopColor="#1E3A8A" />
      </linearGradient>
    </defs>
    <polygon
      points="32,4 54,18 54,46 32,60 10,46 10,18"
      fill="url(#ccGrad)"
      stroke="#7DD3FC"
      strokeWidth="2"
      strokeLinejoin="round"
    />
    <polygon points="32,4 54,18 32,27 10,18" fill="#FFFFFF" fillOpacity="0.28" />
    <polygon points="32,20 46,28 46,42 32,50 18,42 18,28" fill="#0B1120" fillOpacity="0.72" />
    <path d="M24 31 L30 35 L24 39" stroke="#38BDF8" strokeWidth="2.8" strokeLinecap="round" strokeLinejoin="round" />
    <line x1="33" y1="39" x2="40" y2="39" stroke="#E0F2FE" strokeWidth="2.8" strokeLinecap="round" />
  </svg>
);
