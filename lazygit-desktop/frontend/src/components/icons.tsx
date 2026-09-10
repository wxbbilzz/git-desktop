// 内联 SVG 图标，避免引入图标库依赖。
// 统一 24x24 viewBox + currentColor，便于跟随胶囊按钮的文字颜色。

const base = {
  width: 13,
  height: 13,
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 2.2,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
};

export const IconRefresh = () => (
  <svg {...base}>
    <path d="M21 12a9 9 0 1 1-2.6-6.3" />
    <path d="M21 3v6h-6" />
  </svg>
);

export const IconFetch = () => (
  <svg {...base}>
    <path d="M12 3v12" />
    <path d="m7 10 5 5 5-5" />
    <path d="M4 20h16" />
  </svg>
);

export const IconPull = () => (
  <svg {...base}>
    <path d="M12 3v13" />
    <path d="m6 11 6 6 6-6" />
    <path d="M5 21h14" />
  </svg>
);

export const IconPush = () => (
  <svg {...base}>
    <path d="M12 20V6" />
    <path d="m6 11 6-6 6 6" />
    <path d="M5 3h14" />
  </svg>
);

export const IconBranch = () => (
  <svg {...base}>
    <circle cx="6" cy="5" r="2.4" />
    <circle cx="6" cy="19" r="2.4" />
    <circle cx="18" cy="9" r="2.4" />
    <path d="M6 7.4v9.2" />
    <path d="M18 11.4c0 3.4-3 4-6 4" />
  </svg>
);

export const IconPlus = () => (
  <svg {...base}>
    <path d="M12 5v14" />
    <path d="M5 12h14" />
  </svg>
);

export const IconMinus = () => (
  <svg {...base}>
    <path d="M5 12h14" />
  </svg>
);

export const IconCheck = () => (
  <svg {...base}>
    <path d="m4 12 5.5 5.5L20 6.5" />
  </svg>
);

export const IconUndo = () => (
  <svg {...base}>
    <path d="M3 8h11a6 6 0 0 1 0 12H8" />
    <path d="m7 4-4 4 4 4" />
  </svg>
);

export const IconTrash = () => (
  <svg {...base}>
    <path d="M4 7h16" />
    <path d="M9 7V5h6v2" />
    <path d="M6 7l1 13h10l1-13" />
  </svg>
);

export const IconFolder = () => (
  <svg {...base}>
    <path d="M3 7a2 2 0 0 1 2-2h4l2 2.5h8A2 2 0 0 1 21 9.5V17a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z" />
  </svg>
);

export const IconCommit = () => (
  <svg {...base}>
    <circle cx="12" cy="12" r="3.4" />
    <path d="M3 12h5.6" />
    <path d="M15.4 12H21" />
  </svg>
);

export const IconDownload = () => (
  <svg {...base}>
    <path d="M12 3v12" />
    <path d="m7 10 5 5 5-5" />
    <path d="M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2" />
  </svg>
);

export const IconRepo = () => (
  <svg {...base}>
    <path d="M5 4a2 2 0 0 1 2-2h11a1 1 0 0 1 1 1v15a1 1 0 0 1-1 1H7a2 2 0 0 0-2 2z" />
    <path d="M5 18a2 2 0 0 1 2-2h12" />
  </svg>
);

export const IconArrowLeft = () => (
  <svg {...base}>
    <path d="M19 12H5" />
    <path d="m11 6-6 6 6 6" />
  </svg>
);

export const IconHome = () => (
  <svg {...base}>
    <path d="M3 10.5 12 3l9 7.5" />
    <path d="M5.5 9.5V20a1 1 0 0 0 1 1h11a1 1 0 0 0 1-1V9.5" />
  </svg>
);

export const IconUpload = () => (
  <svg {...base}>
    <path d="M12 16V4" />
    <path d="m7 9 5-5 5 5" />
    <path d="M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2" />
  </svg>
);
