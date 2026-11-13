(function injectAppStyles() {
  if (document.getElementById("instant-server-core-styles")) return;
  const st = document.createElement("style");
  st.id = "instant-server-core-styles";
  st.textContent = "    :root,\n    [data-theme=\"light\"] {\n      color-scheme: light;\n      --font-sans: -apple-system, BlinkMacSystemFont, \"SF Pro Text\", \"Inter\", \"Segoe UI Variable Text\", system-ui, sans-serif;\n      --font-display: -apple-system, BlinkMacSystemFont, \"SF Pro Display\", \"Inter\", \"Segoe UI Variable Display\", sans-serif;\n      --font-mono: \"SF Mono\", \"JetBrains Mono\", \"Cascadia Code\", ui-monospace, monospace;\n\n      /* Sleek macOS / Mobbin Alabaster Surfaces (Light) */\n      --mica-bg: #F5F5F7;\n      --mica-tint: radial-gradient(ellipse 80% 50% at 50% -15%, rgba(0, 122, 255, 0.06), transparent 60%);\n      --acrylic-sidebar: rgba(255, 255, 255, 0.82);\n      --card-bg: #FFFFFF;\n      --card-bg-hover: #FAFAFC;\n      --card-bg-secondary: rgba(242, 242, 247, 0.75);\n      --control-bg: #FFFFFF;\n      --control-bg-hover: #F5F5F7;\n      --control-bg-active: #EAEAEE;\n      --input-bg: #FFFFFF;\n      --bg-input: #F2F2F7;\n      --bg-card: #FFFFFF;\n      --bg-card-hover: #F9F9FB;\n\n      /* Flyout / Dropdown Menu Tokens (Light) */\n      --flyout-bg: rgba(255, 255, 255, 0.96);\n      --flyout-border: rgba(0, 0, 0, 0.10);\n      --flyout-shadow: 0 12px 32px rgba(0, 0, 0, 0.12), 0 2px 6px rgba(0, 0, 0, 0.04);\n      --flyout-item-hover: rgba(0, 122, 255, 0.08);\n      --flyout-item-selected: rgba(0, 122, 255, 0.12);\n\n      /* Scrollbar Tokens (Light) */\n      --scroll-track-active: rgba(0, 0, 0, 0.04);\n      --scroll-thumb-idle: rgba(0, 0, 0, 0.15);\n      --scroll-thumb-active: rgba(0, 0, 0, 0.38);\n\n      /* Borders & Elevation */\n      --radius: 10px;\n      --radius-lg: 14px;\n      --radius-md: 10px;\n      --radius-sm: 8px;\n      --border-card: rgba(0, 0, 0, 0.075);\n      --border-subtle: rgba(0, 0, 0, 0.075);\n      --border-strong: rgba(0, 0, 0, 0.14);\n      --border-control: rgba(0, 0, 0, 0.12);\n      --border-control-bottom: rgba(0, 0, 0, 0.16);\n      --shadow-card: 0 1px 3px rgba(0, 0, 0, 0.03), 0 1px 2px rgba(0, 0, 0, 0.02);\n      --shadow-control: 0 1px 2px rgba(0, 0, 0, 0.04);\n      --shadow-setup: 0 20px 50px rgba(0, 0, 0, 0.09), 0 1px 3px rgba(0, 0, 0, 0.04);\n\n      /* Text Hierarchy */\n      --text-primary: #1D1D1F;\n      --text-secondary: #515154;\n      --text-tertiary: #86868B;\n\n      /* Accent & Semantic Tokens */\n      --accent: #007AFF;\n      --accent-hover: #006CE6;\n      --accent-active: #005BB5;\n      --accent-text: #FFFFFF;\n      --accent-subtle: rgba(0, 122, 255, 0.08);\n      --accent-soft: rgba(0, 122, 255, 0.10);\n\n      --success: #1D8348;\n      --success-bg: rgba(40, 180, 99, 0.12);\n      --warning: #B76E00;\n      --warning-bg: rgba(245, 166, 35, 0.14);\n      --danger: #D70015;\n      --danger-bg: rgba(255, 59, 48, 0.10);\n      --nav-hover: rgba(0, 0, 0, 0.04);\n      --nav-active: rgba(0, 122, 255, 0.10);\n      --divider: rgba(0, 0, 0, 0.07);\n    }\n\n    [data-theme=\"dark\"] {\n      color-scheme: dark;\n      /* Sleek macOS Sequoia / Raycast / OrbStack Obsidian Surfaces (Dark) */\n      --mica-bg: #0B0C0E;\n      --mica-tint: radial-gradient(ellipse 80% 45% at 50% -15%, rgba(10, 132, 255, 0.075), transparent 60%);\n      --acrylic-sidebar: rgba(18, 19, 23, 0.82);\n      --card-bg: #141519;\n      --card-bg-hover: #1A1C22;\n      --card-bg-secondary: #101115;\n      --control-bg: #1C1E24;\n      --control-bg-hover: #24272F;\n      --control-bg-active: #16181D;\n      --input-bg: #121317;\n      --bg-input: #121317;\n      --bg-card: #141519;\n      --bg-card-hover: #1A1C22;\n\n      /* Flyout / Dropdown Menu Tokens (Dark) */\n      --flyout-bg: #1C1E24;\n      --flyout-border: rgba(255, 255, 255, 0.12);\n      --flyout-shadow: 0 16px 40px rgba(0, 0, 0, 0.65), inset 0 1px 0 rgba(255, 255, 255, 0.08);\n      --flyout-item-hover: rgba(255, 255, 255, 0.07);\n      --flyout-item-selected: rgba(10, 132, 255, 0.18);\n\n      /* Scrollbar Tokens (Dark) */\n      --scroll-track-active: rgba(255, 255, 255, 0.035);\n      --scroll-thumb-idle: rgba(255, 255, 255, 0.14);\n      --scroll-thumb-active: rgba(255, 255, 255, 0.36);\n\n      /* Borders & Elevation */\n      --radius: 10px;\n      --radius-lg: 14px;\n      --radius-md: 10px;\n      --radius-sm: 8px;\n      --border-card: rgba(255, 255, 255, 0.075);\n      --border-subtle: rgba(255, 255, 255, 0.075);\n      --border-strong: rgba(255, 255, 255, 0.14);\n      --border-control: rgba(255, 255, 255, 0.10);\n      --border-control-bottom: rgba(255, 255, 255, 0.14);\n      --shadow-card: 0 2px 8px rgba(0, 0, 0, 0.28), inset 0 1px 0 rgba(255, 255, 255, 0.04);\n      --shadow-control: 0 1px 2px rgba(0, 0, 0, 0.32), inset 0 1px 0 rgba(255, 255, 255, 0.05);\n      --shadow-setup: 0 24px 60px rgba(0, 0, 0, 0.55), inset 0 1px 0 rgba(255, 255, 255, 0.06);\n\n      /* Text Hierarchy */\n      --text-primary: #F4F4F6;\n      --text-secondary: #A1A1AA;\n      --text-tertiary: #71717A;\n\n      /* Accent & Semantic Tokens */\n      --accent: #0A84FF;\n      --accent-hover: #2492FF;\n      --accent-active: #0070E0;\n      --accent-text: #FFFFFF;\n      --accent-subtle: rgba(10, 132, 255, 0.12);\n      --accent-soft: rgba(10, 132, 255, 0.15);\n\n      --success: #30D158;\n      --success-bg: rgba(48, 209, 88, 0.14);\n      --warning: #FF9F0A;\n      --warning-bg: rgba(255, 159, 10, 0.14);\n      --danger: #FF453A;\n      --danger-bg: rgba(255, 69, 58, 0.14);\n      --nav-hover: rgba(255, 255, 255, 0.05);\n      --nav-active: rgba(10, 132, 255, 0.15);\n      --divider: rgba(255, 255, 255, 0.07);\n    }\n\n    * {\n      box-sizing: border-box;\n      margin: 0;\n      padding: 0;\n    }\n\n    *::-webkit-scrollbar {\n      width: 11px;\n      height: 11px;\n      background: transparent;\n    }\n    *::-webkit-scrollbar-track {\n      background: transparent;\n      border-radius: 999px;\n      margin: 4px 0;\n    }\n    *::-webkit-scrollbar-corner {\n      background: transparent;\n    }\n    *::-webkit-scrollbar-thumb {\n      background-color: var(--scroll-thumb-idle);\n      border-radius: 999px;\n      border: 2.5px solid transparent;\n      background-clip: padding-box;\n      min-height: 44px;\n    }\n    *::-webkit-scrollbar-thumb:hover {\n      background-color: var(--scroll-thumb-active);\n    }\n    *::-webkit-scrollbar-thumb:active {\n      background-color: var(--accent);\n    }\n\n    .select-all-checkbox-pill {\n      display: inline-flex;\n      align-items: center;\n      gap: 9px;\n      height: 32px;\n      padding: 0 13px;\n      border-radius: var(--radius-sm);\n      border: 1.5px solid var(--accent);\n      background: var(--accent-subtle);\n      color: var(--text-primary);\n      font-family: var(--font-sans);\n      font-size: 12.5px;\n      font-weight: 600;\n      cursor: pointer;\n      user-select: none;\n      transition: background 0.14s ease, border-color 0.14s ease, transform 0.1s ease;\n    }\n    .select-all-checkbox-pill:hover {\n      background: var(--accent-soft);\n    }\n    .select-all-checkbox-pill:active {\n      transform: scale(0.98);\n    }\n    .select-all-checkbox-pill input[type=\"checkbox\"] {\n      width: 16px;\n      height: 16px;\n      accent-color: var(--accent);\n      cursor: pointer;\n      pointer-events: none;\n      margin: 0;\n    }\n\n    html, body {\n      height: 100%;\n      font-family: var(--font-sans);\n      font-size: 13.5px;\n      line-height: 1.45;\n      color: var(--text-primary);\n      background-color: var(--mica-bg);\n      background-image: var(--mica-tint);\n      -webkit-font-smoothing: antialiased;\n      overflow: hidden;\n      user-select: none;\n    }\n\n    .app-window {\n      display: flex;\n      flex-direction: column;\n      width: 100vw;\n      height: 100vh;\n      overflow: hidden;\n      transition: background-color 0.25s ease;\n    }\n\n    /* Unified Native Titlebar (Zero Fake OS Buttons, Zero Version Clutter) */\n    .window-titlebar {\n      height: 50px;\n      display: flex;\n      align-items: center;\n      justify-content: space-between;\n      padding: 0 18px;\n      background: var(--acrylic-sidebar);\n      backdrop-filter: blur(28px) saturate(180%);\n      -webkit-backdrop-filter: blur(28px) saturate(180%);\n      border-bottom: 1px solid var(--divider);\n      flex-shrink: 0;\n      gap: 16px;\n      z-index: 20;\n    }\n\n    .titlebar-left {\n      display: flex;\n      align-items: center;\n      gap: 10px;\n      min-width: 170px;\n    }\n\n    .titlebar-back {\n      width: 30px;\n      height: 30px;\n      border-radius: var(--radius-sm);\n      border: 1px solid transparent;\n      background: transparent;\n      color: var(--text-primary);\n      display: inline-flex;\n      align-items: center;\n      justify-content: center;\n      cursor: pointer;\n      transition: background 0.14s ease, transform 0.1s ease;\n    }\n    .titlebar-back:hover {\n      background: var(--nav-hover);\n    }\n    .titlebar-back:active {\n      transform: scale(0.93);\n    }\n\n    .titlebar-logo {\n      width: 24px;\n      height: 24px;\n      border-radius: 6px;\n      background: #001E36;\n      border: 1.5px solid #31A8FF;\n      color: #66C4FF;\n      font-weight: 800;\n      font-size: 11px;\n      display: inline-flex;\n      align-items: center;\n      justify-content: center;\n      flex-shrink: 0;\n    }\n\n    .app-brand-title {\n      font-size: 13.5px;\n      font-weight: 600;\n      color: var(--text-primary);\n      letter-spacing: -0.01em;\n    }\n\n    .titlebar-center {\n      flex: 1;\n      max-width: 440px;\n      display: flex;\n      align-items: center;\n      position: relative;\n      transition: opacity 0.2s ease, transform 0.2s ease;\n    }\n    .titlebar-center.hidden-in-setup {\n      opacity: 0;\n      pointer-events: none;\n      transform: translateY(-4px);\n    }\n\n    .titlebar-search-input {\n      width: 100%;\n      height: 32px;\n      padding: 0 34px 0 12px;\n      border-radius: var(--radius-sm);\n      border: 1px solid var(--border-control);\n      background: var(--input-bg);\n      color: var(--text-primary);\n      font-family: var(--font-sans);\n      font-size: 13px;\n      outline: none;\n      transition: border-color 0.15s ease, background 0.15s ease, box-shadow 0.15s ease;\n    }\n    .titlebar-search-input:focus {\n      border-color: var(--accent);\n      box-shadow: 0 0 0 3px var(--accent-subtle);\n      background: var(--control-bg);\n    }\n    .titlebar-search-icon {\n      position: absolute;\n      right: 10px;\n      top: 50%;\n      transform: translateY(-50%);\n      color: var(--text-tertiary);\n      pointer-events: none;\n      font-size: 13px;\n    }\n\n    .titlebar-right {\n      display: flex;\n      align-items: center;\n      justify-content: flex-end;\n      gap: 8px;\n      min-width: 170px;\n    }\n\n    @keyframes fadeSlideUp {\n      from {\n        opacity: 0;\n        transform: translateY(6px);\n      }\n      to {\n        opacity: 1;\n        transform: translateY(0);\n      }\n    }\n\n    .animate-tab-switch {\n      animation: fadeSlideUp 0.22s cubic-bezier(0.22, 1, 0.36, 1) forwards;\n    }\n\n    .tab-flow-stack {\n      display: flex;\n      flex-direction: column;\n      gap: 14px;\n    }\n\n    /* MODE 1: First-Run Setup Wizard Window */\n    .setup-shell {\n      flex: 1;\n      min-height: 0;\n      display: flex;\n      flex-direction: column;\n      align-items: center;\n      justify-content: center;\n      padding: 18px 24px;\n      overflow: hidden;\n    }\n\n    .setup-window-card {\n      width: 100%;\n      max-width: 880px;\n      max-height: 100%;\n      flex: 1;\n      min-height: 0;\n      background: var(--card-bg);\n      border: 1px solid var(--border-card);\n      border-radius: var(--radius-lg);\n      box-shadow: var(--shadow-setup);\n      display: flex;\n      flex-direction: column;\n      overflow: hidden;\n    }\n\n    .setup-top-tabs {\n      flex-shrink: 0;\n      min-height: 50px;\n      display: flex;\n      align-items: center;\n      gap: 4px;\n      padding: 8px 18px 0;\n      border-bottom: 1px solid var(--divider);\n      background: var(--card-bg-secondary);\n      overflow-x: auto;\n      overflow-y: hidden;\n    }\n\n    .setup-tab-btn {\n      flex-shrink: 0;\n      position: relative;\n      display: inline-flex;\n      align-items: center;\n      gap: 8px;\n      padding: 9px 16px 11px;\n      border: none;\n      background: transparent;\n      color: var(--text-secondary);\n      font-family: var(--font-sans);\n      font-size: 13px;\n      font-weight: 500;\n      cursor: pointer;\n      border-radius: var(--radius-sm) var(--radius-sm) 0 0;\n      transition: color 0.16s ease, background 0.16s ease;\n      white-space: nowrap;\n    }\n    .setup-tab-btn:hover {\n      color: var(--text-primary);\n      background: var(--nav-hover);\n    }\n    .setup-tab-btn.active {\n      color: var(--text-primary);\n      font-weight: 600;\n    }\n    .setup-tab-btn.active::after {\n      content: \"\";\n      position: absolute;\n      left: 14px;\n      right: 14px;\n      bottom: 0;\n      height: 2.5px;\n      border-radius: 3px 3px 0 0;\n      background: var(--accent);\n    }\n\n    .setup-tab-step-dot {\n      width: 20px;\n      height: 20px;\n      border-radius: 50%;\n      font-size: 11px;\n      font-weight: 600;\n      display: inline-flex;\n      align-items: center;\n      justify-content: center;\n      background: var(--control-bg);\n      border: 1px solid var(--border-control);\n      color: var(--text-secondary);\n      transition: background 0.18s ease, color 0.18s ease, border-color 0.18s ease;\n    }\n    .setup-tab-btn.active .setup-tab-step-dot {\n      background: var(--accent);\n      color: var(--accent-text);\n      border-color: var(--accent);\n    }\n\n    .setup-body {\n      flex: 1;\n      min-height: 0;\n      overflow-y: auto;\n      overscroll-behavior: contain;\n      -webkit-overflow-scrolling: touch;\n      will-change: scroll-position;\n      padding: 24px 28px;\n      display: flex;\n      flex-direction: column;\n      gap: 16px;\n    }\n\n    .setup-header-title {\n      font-family: var(--font-display);\n      font-size: 22px;\n      font-weight: 600;\n      letter-spacing: -0.02em;\n      color: var(--text-primary);\n      margin-bottom: 4px;\n    }\n    .setup-header-desc {\n      font-size: 13.5px;\n      color: var(--text-secondary);\n    }\n\n    .setup-footer {\n      flex-shrink: 0;\n      display: flex;\n      align-items: center;\n      justify-content: space-between;\n      padding: 14px 28px;\n      border-top: 1px solid var(--divider);\n      background: var(--card-bg-secondary);\n      gap: 12px;\n      flex-wrap: wrap;\n    }\n    .setup-footer-right {\n      display: flex;\n      align-items: center;\n      gap: 10px;\n      flex-wrap: wrap;\n    }\n\n    /* Animated Advanced Options Expander */\n    .expander-wrap {\n      border-radius: var(--radius);\n      border: 1px solid var(--border-card);\n      background: var(--card-bg-secondary);\n      transition: border-color 0.2s ease;\n      flex-shrink: 0;\n    }\n    .expander-header {\n      width: 100%;\n      padding: 13px 18px;\n      background: transparent;\n      border: none;\n      border-radius: var(--radius);\n      color: var(--text-primary);\n      font-family: var(--font-sans);\n      font-size: 13.5px;\n      font-weight: 600;\n      display: flex;\n      align-items: center;\n      justify-content: space-between;\n      cursor: pointer;\n      transition: background 0.15s ease;\n      text-align: left;\n    }\n    .expander-header:hover {\n      background: var(--nav-hover);\n    }\n    .expander-chevron {\n      display: inline-flex;\n      align-items: center;\n      justify-content: center;\n      width: 24px;\n      height: 24px;\n      color: var(--text-secondary);\n      transition: transform 0.32s cubic-bezier(0.25, 1, 0.5, 1);\n      font-size: 14px;\n    }\n    .expander-wrap.open .expander-chevron {\n      transform: rotate(180deg);\n    }\n    .expander-content-grid {\n      display: grid;\n      grid-template-rows: 0fr;\n      opacity: 0;\n      transition:\n        grid-template-rows 0.32s cubic-bezier(0.25, 1, 0.5, 1),\n        opacity 0.26s cubic-bezier(0.25, 1, 0.5, 1);\n      will-change: grid-template-rows, opacity;\n    }\n    .expander-wrap.open .expander-content-grid {\n      grid-template-rows: 1fr;\n      opacity: 1;\n    }\n    .expander-content-inner {\n      min-height: 0;\n      overflow: hidden;\n    }\n    .expander-wrap.open.dropdown-open-inside .expander-content-inner {\n      overflow: visible;\n    }\n    .expander-body {\n      padding: 0 14px;\n      border-top: 1px solid transparent;\n      display: flex;\n      flex-direction: column;\n      gap: 8px;\n      transform: translateY(-4px);\n      transition:\n        padding 0.32s cubic-bezier(0.25, 1, 0.5, 1),\n        transform 0.32s cubic-bezier(0.25, 1, 0.5, 1),\n        border-color 0.26s ease;\n    }\n    .expander-wrap.open .expander-body {\n      padding: 12px 14px 14px;\n      border-top-color: var(--divider);\n      transform: translateY(0);\n    }\n\n    /* MODE 2: Settings Two-Column Layout */\n    .settings-shell {\n      display: flex;\n      flex: 1;\n      min-height: 0;\n      overflow: hidden;\n    }\n\n    .nav-sidebar {\n      width: 248px;\n      flex-shrink: 0;\n      background: var(--acrylic-sidebar);\n      backdrop-filter: blur(28px) saturate(180%);\n      -webkit-backdrop-filter: blur(28px) saturate(180%);\n      padding: 14px 10px 16px;\n      display: flex;\n      flex-direction: column;\n      gap: 4px;\n      border-right: 1px solid var(--divider);\n      overflow-y: auto;\n    }\n\n    .nav-list {\n      display: flex;\n      flex-direction: column;\n      gap: 3px;\n    }\n\n    .nav-item {\n      position: relative;\n      display: flex;\n      align-items: center;\n      gap: 11px;\n      height: 36px;\n      padding: 0 12px;\n      border-radius: var(--radius-sm);\n      border: none;\n      background: transparent;\n      color: var(--text-secondary);\n      font-family: var(--font-sans);\n      font-size: 13px;\n      font-weight: 500;\n      cursor: pointer;\n      text-align: left;\n      transition: background 0.14s ease, color 0.14s ease;\n    }\n    .nav-item:hover {\n      background: var(--nav-hover);\n      color: var(--text-primary);\n    }\n    .nav-item.active {\n      background: var(--nav-active);\n      color: var(--accent);\n      font-weight: 600;\n    }\n    .nav-icon {\n      width: 18px;\n      height: 18px;\n      display: inline-flex;\n      align-items: center;\n      justify-content: center;\n      font-size: 14px;\n      flex-shrink: 0;\n    }\n\n    .content-area {\n      flex: 1;\n      min-height: 0;\n      overflow-y: auto;\n      overscroll-behavior: contain;\n      -webkit-overflow-scrolling: touch;\n      will-change: scroll-position;\n      padding: 24px 36px 44px;\n    }\n\n    .content-inner {\n      max-width: 900px;\n      margin: 0 auto;\n      display: flex;\n      flex-direction: column;\n      gap: 16px;\n    }\n\n    .page-header {\n      display: flex;\n      align-items: center;\n      justify-content: space-between;\n      margin-bottom: 4px;\n      gap: 12px;\n    }\n\n    .breadcrumb-bar {\n      display: flex;\n      align-items: center;\n      gap: 8px;\n      font-family: var(--font-display);\n      font-size: 24px;\n      font-weight: 600;\n      letter-spacing: -0.02em;\n      color: var(--text-primary);\n    }\n    .breadcrumb-link {\n      color: var(--text-secondary);\n      cursor: pointer;\n      transition: color 0.12s ease;\n    }\n    .breadcrumb-link:hover {\n      color: var(--text-primary);\n    }\n    .breadcrumb-sep {\n      color: var(--text-tertiary);\n      font-size: 18px;\n      font-weight: 400;\n    }\n\n    .system-summary-banner {\n      display: flex;\n      align-items: center;\n      justify-content: space-between;\n      gap: 20px;\n      padding: 18px 22px;\n      border-radius: var(--radius-lg);\n      background: var(--card-bg);\n      border: 1px solid var(--border-card);\n      box-shadow: var(--shadow-card);\n      flex-wrap: wrap;\n    }\n    .summary-left {\n      display: flex;\n      flex-direction: column;\n      gap: 2px;\n    }\n    .summary-title {\n      font-family: var(--font-display);\n      font-size: 19px;\n      font-weight: 600;\n      color: var(--text-primary);\n    }\n    .summary-subtitle {\n      font-size: 13px;\n      color: var(--text-secondary);\n    }\n    .summary-pills {\n      display: flex;\n      align-items: center;\n      gap: 8px;\n      flex-wrap: wrap;\n    }\n    .summary-pill {\n      padding: 6px 12px;\n      border-radius: var(--radius-sm);\n      background: var(--card-bg-secondary);\n      border: 1px solid var(--border-card);\n      font-size: 12px;\n      color: var(--text-secondary);\n    }\n    .summary-pill strong {\n      color: var(--text-primary);\n      font-weight: 600;\n    }\n\n    /* Setting Cards */\n    .section-label {\n      font-size: 12.5px;\n      font-weight: 600;\n      color: var(--text-secondary);\n      margin: 4px 0 0;\n    }\n\n    .card-stack {\n      display: flex;\n      flex-direction: column;\n      gap: 8px;\n    }\n\n    .win-card {\n      display: flex;\n      align-items: center;\n      justify-content: space-between;\n      gap: 16px;\n      padding: 14px 18px;\n      border-radius: var(--radius);\n      background: var(--card-bg);\n      border: 1px solid var(--border-card);\n      box-shadow: var(--shadow-card);\n      transition: background 0.14s ease, border-color 0.14s ease, transform 0.1s ease;\n    }\n    .win-card.clickable {\n      cursor: pointer;\n    }\n    .win-card.clickable:hover {\n      background: var(--card-bg-hover);\n      border-color: var(--border-strong);\n    }\n    .win-card.clickable:active {\n      transform: scale(0.996);\n    }\n    .win-card.selected-accent {\n      border-color: var(--accent);\n      background: var(--accent-subtle);\n    }\n\n    .card-left {\n      display: flex;\n      align-items: center;\n      gap: 14px;\n      min-width: 0;\n      flex: 1;\n    }\n    .card-icon {\n      width: 28px;\n      height: 28px;\n      border-radius: 7px;\n      background: var(--card-bg-secondary);\n      border: 1px solid var(--border-card);\n      display: inline-flex;\n      align-items: center;\n      justify-content: center;\n      font-size: 15px;\n      color: var(--text-primary);\n      flex-shrink: 0;\n    }\n    .card-text {\n      min-width: 0;\n    }\n    .card-title {\n      font-size: 13.5px;\n      font-weight: 500;\n      color: var(--text-primary);\n      display: flex;\n      align-items: center;\n      gap: 8px;\n      flex-wrap: wrap;\n    }\n    .card-desc {\n      font-size: 12px;\n      color: var(--text-secondary);\n      margin-top: 2px;\n    }\n    .card-right {\n      display: flex;\n      align-items: center;\n      gap: 10px;\n      flex-shrink: 0;\n    }\n\n    /* Self-Hosted App Store Toolbar & Grid */\n    .appstore-toolbar {\n      display: flex;\n      flex-direction: column;\n      gap: 10px;\n      padding: 14px 16px;\n      border-radius: var(--radius);\n      background: var(--card-bg-secondary);\n      border: 1px solid var(--border-card);\n    }\n    .appstore-top-row {\n      display: flex;\n      align-items: center;\n      justify-content: space-between;\n      gap: 10px;\n      flex-wrap: wrap;\n    }\n    .appstore-filter-pills {\n      display: flex;\n      align-items: center;\n      gap: 6px;\n      flex-wrap: wrap;\n    }\n    .filter-pill {\n      height: 28px;\n      padding: 0 12px;\n      border-radius: 999px;\n      border: 1px solid var(--border-control);\n      background: var(--control-bg);\n      color: var(--text-secondary);\n      font-family: var(--font-sans);\n      font-size: 12px;\n      font-weight: 500;\n      cursor: pointer;\n      transition: background 0.14s ease, color 0.14s ease, border-color 0.14s ease;\n    }\n    .filter-pill:hover {\n      background: var(--control-bg-hover);\n      color: var(--text-primary);\n    }\n    .filter-pill.active {\n      background: var(--accent);\n      color: var(--accent-text);\n      border-color: var(--accent);\n      font-weight: 600;\n    }\n\n    .appstore-grid {\n      display: grid;\n      grid-template-columns: repeat(auto-fill, minmax(330px, 1fr));\n      gap: 8px;\n    }\n    .app-card {\n      contain: layout paint;\n      display: flex;\n      align-items: flex-start;\n      justify-content: space-between;\n      gap: 12px;\n      padding: 13px 15px;\n      border-radius: var(--radius);\n      background: var(--card-bg);\n      border: 1px solid var(--border-card);\n      box-shadow: var(--shadow-card);\n      cursor: pointer;\n      transition: background 0.14s ease, border-color 0.14s ease, transform 0.1s ease;\n    }\n    .app-card:hover {\n      background: var(--card-bg-hover);\n      border-color: var(--border-strong);\n    }\n    .app-card:active {\n      transform: scale(0.994);\n    }\n    .app-card.checked {\n      border-color: var(--accent);\n      background: var(--accent-subtle);\n    }\n    .app-checkbox {\n      width: 17px;\n      height: 17px;\n      accent-color: var(--accent);\n      margin-top: 2px;\n      flex-shrink: 0;\n      pointer-events: none;\n    }\n\n    /* Controls */\n    .win-btn {\n      height: 33px;\n      padding: 0 14px;\n      border-radius: var(--radius-sm);\n      border: 1px solid var(--border-control);\n      background: var(--control-bg);\n      color: var(--text-primary);\n      font-family: var(--font-sans);\n      font-size: 12.5px;\n      font-weight: 500;\n      box-shadow: var(--shadow-control);\n      cursor: pointer;\n      display: inline-flex;\n      align-items: center;\n      justify-content: center;\n      gap: 7px;\n      transition: background 0.14s ease, transform 0.1s ease, border-color 0.14s ease;\n    }\n    .win-btn:hover:not(:disabled) {\n      background: var(--control-bg-hover);\n      border-color: var(--border-strong);\n    }\n    .win-btn:active:not(:disabled) {\n      transform: scale(0.97);\n      background: var(--control-bg-active);\n    }\n    .win-btn:disabled {\n      opacity: 0.42;\n      cursor: not-allowed;\n    }\n\n    .win-btn-sm {\n      height: 28px;\n      padding: 0 11px;\n      font-size: 12px;\n    }\n\n    .win-btn-accent {\n      background: var(--accent);\n      color: var(--accent-text);\n      border-color: rgba(255, 255, 255, 0.16);\n      font-weight: 600;\n    }\n    .win-btn-accent:hover:not(:disabled) {\n      background: var(--accent-hover);\n    }\n    .win-btn-accent:active:not(:disabled) {\n      background: var(--accent-active);\n      transform: scale(0.97);\n    }\n\n    .win-btn-large {\n      height: 38px;\n      padding: 0 24px;\n      font-size: 13.5px;\n    }\n\n    .win-input {\n      height: 33px;\n      min-width: 200px;\n      padding: 0 11px;\n      border-radius: var(--radius-sm);\n      border: 1px solid var(--border-control);\n      background: var(--input-bg);\n      color: var(--text-primary);\n      font-family: var(--font-sans);\n      font-size: 13px;\n      outline: none;\n      transition: border-color 0.14s ease, box-shadow 0.14s ease;\n    }\n    .win-input:focus {\n      border-color: var(--accent);\n      box-shadow: 0 0 0 3px var(--accent-subtle);\n      background: var(--control-bg);\n    }\n\n    /* Custom ComboBox Dropdown */\n    .win-combobox {\n      position: relative;\n      min-width: 240px;\n      user-select: none;\n    }\n    .win-combobox-trigger {\n      width: 100%;\n      height: 33px;\n      padding: 0 12px;\n      border-radius: var(--radius-sm);\n      border: 1px solid var(--border-control);\n      background: var(--control-bg);\n      color: var(--text-primary);\n      font-family: var(--font-sans);\n      font-size: 12.5px;\n      font-weight: 500;\n      box-shadow: var(--shadow-control);\n      cursor: pointer;\n      display: flex;\n      align-items: center;\n      justify-content: space-between;\n      gap: 12px;\n      transition: background 0.14s ease, border-color 0.14s ease;\n    }\n    .win-combobox-trigger:hover {\n      background: var(--control-bg-hover);\n    }\n    .win-combobox.open .win-combobox-trigger {\n      border-color: var(--accent);\n    }\n    .win-combobox-chevron {\n      font-size: 12px;\n      color: var(--text-secondary);\n      transition: transform 0.18s ease;\n    }\n    .win-combobox.open .win-combobox-chevron {\n      transform: rotate(180deg);\n    }\n    .win-combobox {\n      max-width: 100%;\n    }\n    #minecraftCustomizationStudioCard .win-combobox {\n      min-width: 0;\n      width: 100%;\n      flex: 1 1 0%;\n    }\n    #minecraftCustomizationStudioCard > div > div {\n      min-width: 0;\n    }\n\n    /* Smooth Floating Value Box for Range Sliders */\n    .win-slider-wrap {\n      position: relative;\n      display: inline-flex;\n      align-items: center;\n      padding: 6px 0;\n    }\n    .win-slider-wrap input[type=\"range\"] {\n      margin: 0;\n      cursor: pointer;\n    }\n    .win-slider-bubble {\n      position: absolute;\n      bottom: calc(100% + 4px);\n      left: 50%;\n      transform: translateX(-50%) translateY(6px) scale(0.88);\n      background: var(--dropdown-bg);\n      color: var(--text-primary);\n      border: 1px solid var(--accent);\n      border-radius: 6px;\n      padding: 3px 8px;\n      font-family: var(--font-sans);\n      font-size: 11.5px;\n      font-weight: 600;\n      line-height: 1.2;\n      white-space: nowrap;\n      pointer-events: none;\n      box-shadow: 0 6px 16px rgba(0, 0, 0, 0.28);\n      opacity: 0;\n      visibility: hidden;\n      transition: opacity 0.18s cubic-bezier(0.16, 1, 0.3, 1),\n                  transform 0.18s cubic-bezier(0.16, 1, 0.3, 1),\n                  left 0.05s linear,\n                  visibility 0.18s;\n      z-index: 60;\n    }\n    .win-slider-wrap.active .win-slider-bubble {\n      opacity: 1;\n      visibility: visible;\n      transform: translateX(-50%) translateY(0) scale(1);\n    }\n    .win-combobox-label {\n      overflow: hidden;\n      text-overflow: ellipsis;\n      white-space: nowrap;\n    }\n    .win-combobox-menu {\n      position: absolute;\n      top: calc(100% + 5px);\n      left: 0;\n      right: auto;\n      min-width: 100%;\n      max-width: min(88vw, 390px);\n      max-height: 260px;\n      overflow-y: auto;\n      padding: 5px;\n      border-radius: var(--radius);\n      background: var(--flyout-bg);\n      backdrop-filter: blur(24px);\n      border: 1px solid var(--flyout-border);\n      box-shadow: var(--flyout-shadow);\n      z-index: 120;\n      display: none;\n      flex-direction: column;\n      gap: 2px;\n      animation: fadeSlideUp 0.14s cubic-bezier(0.22, 1, 0.36, 1) forwards;\n    }\n    .card-right .win-combobox-menu,\n    .win-combobox.align-right .win-combobox-menu {\n      left: auto;\n      right: 0;\n    }\n    .win-combobox.drop-up .win-combobox-menu {\n      top: auto;\n      bottom: calc(100% + 5px);\n    }\n    .win-combobox.open .win-combobox-menu {\n      display: flex;\n    }\n    .win-card.has-open-dropdown {\n      z-index: 95 !important;\n      overflow: visible !important;\n      position: relative;\n    }\n    .win-combobox-option {\n      position: relative;\n      width: 100%;\n      padding: 8px 12px 8px 14px;\n      border: none;\n      border-radius: 6px;\n      background: transparent;\n      color: var(--text-primary);\n      font-family: var(--font-sans);\n      font-size: 12.5px;\n      line-height: 1.35;\n      text-align: left;\n      cursor: pointer;\n      display: flex;\n      align-items: center;\n      transition: background 0.12s ease;\n      white-space: normal;\n      word-break: break-word;\n    }\n    input[type=\"number\"].win-input,\n    .storage-num-input {\n      padding-right: 8px !important;\n      min-width: 96px;\n      font-variant-numeric: tabular-nums;\n    }\n    .win-combobox-option:hover {\n      background: var(--flyout-item-hover);\n    }\n    .win-combobox-option.selected {\n      background: var(--flyout-item-selected);\n      color: var(--accent);\n      font-weight: 600;\n    }\n\n    /* Sleek macOS Toggle Switch */\n    .win-switch {\n      display: inline-flex;\n      align-items: center;\n      gap: 10px;\n      cursor: pointer;\n    }\n    .win-switch-label {\n      font-size: 12.5px;\n      color: var(--text-secondary);\n      min-width: 24px;\n      text-align: right;\n    }\n    .win-switch-track {\n      width: 40px;\n      height: 22px;\n      border-radius: 999px;\n      border: 1px solid var(--border-strong);\n      background: var(--bg-input);\n      position: relative;\n      transition: background 0.18s ease, border-color 0.18s ease;\n    }\n    .win-switch-thumb {\n      width: 16px;\n      height: 16px;\n      border-radius: 50%;\n      background: #FFFFFF;\n      box-shadow: 0 1px 3px rgba(0, 0, 0, 0.35);\n      position: absolute;\n      top: 2px;\n      left: 2px;\n      transition: transform 0.2s cubic-bezier(0.22, 1, 0.36, 1);\n    }\n    .win-switch.on .win-switch-track {\n      background: var(--accent);\n      border-color: var(--accent);\n    }\n    .win-switch.on .win-switch-thumb {\n      transform: translateX(18px);\n    }\n\n    /* Badges */\n    .badge {\n      display: inline-flex;\n      align-items: center;\n      padding: 2px 8px;\n      border-radius: 999px;\n      font-size: 11px;\n      font-weight: 600;\n    }\n    .badge-accent {\n      background: var(--accent-subtle);\n      color: var(--accent);\n    }\n    .badge-success {\n      background: var(--success-bg);\n      color: var(--success);\n    }\n    .badge-warning {\n      background: var(--warning-bg);\n      color: var(--warning);\n    }\n    .badge-danger {\n      background: var(--danger-bg);\n      color: var(--danger);\n    }\n\n    .notice-banner {\n      padding: 13px 16px;\n      border-radius: var(--radius);\n      border: 1px solid var(--danger);\n      background: var(--danger-bg);\n      display: flex;\n      flex-direction: column;\n      gap: 8px;\n      font-size: 13px;\n      flex-shrink: 0;\n    }\n    .notice-banner-warning {\n      border-color: var(--warning);\n      background: var(--warning-bg);\n    }\n\n    /* Flashing Progress & Live Logs Card */\n    .flash-panel {\n      padding: 18px 20px;\n      border-radius: var(--radius-lg);\n      background: var(--card-bg);\n      border: 1px solid var(--border-card);\n      box-shadow: var(--shadow-card);\n      display: flex;\n      flex-direction: column;\n      gap: 12px;\n      flex-shrink: 0;\n    }\n    .progress-track {\n      width: 100%;\n      height: 7px;\n      border-radius: 999px;\n      background: var(--card-bg-secondary);\n      border: 1px solid var(--border-card);\n      overflow: hidden;\n    }\n    .progress-fill {\n      height: 100%;\n      width: 0%;\n      border-radius: 999px;\n      background: var(--accent);\n      transition: width 0.25s ease;\n    }\n    .flash-terminal-wrap {\n      position: relative;\n      display: flex;\n      flex-direction: column;\n    }\n    .flash-log-box {\n      background: #090A0D;\n      border: 1px solid var(--border-card);\n      border-radius: var(--radius-sm);\n      padding: 12px 14px;\n      font-family: var(--font-mono);\n      font-size: 12px;\n      line-height: 1.6;\n      color: #D4DCE8;\n      max-height: 210px;\n      overflow-y: auto;\n      user-select: text;\n      overscroll-behavior: contain;\n    }\n    .term-log-line {\n      white-space: pre-wrap;\n      word-break: break-word;\n      padding: 1px 0;\n    }\n    .term-live-progress {\n      margin-top: 6px;\n      padding: 6px 10px;\n      border-radius: 6px;\n      background: rgba(10, 132, 255, 0.14);\n      border: 1px solid rgba(10, 132, 255, 0.30);\n      color: #5AC8FA;\n      font-family: var(--font-mono);\n      font-size: 12px;\n      font-weight: 600;\n      white-space: pre-wrap;\n      word-break: break-word;\n    }\n    .resume-scroll-pill {\n      position: absolute;\n      right: 14px;\n      bottom: 12px;\n      padding: 5px 12px;\n      border-radius: 999px;\n      background: var(--accent);\n      color: var(--accent-text);\n      border: none;\n      font-family: var(--font-sans);\n      font-size: 11.5px;\n      font-weight: 600;\n      cursor: pointer;\n      box-shadow: 0 4px 14px rgba(0, 0, 0, 0.35);\n      z-index: 5;\n    }\n    .win-btn-danger {\n      background: var(--danger-bg);\n      border-color: var(--danger);\n      color: var(--danger);\n      font-weight: 600;\n    }\n    .win-btn-danger:hover:not(:disabled) {\n      background: var(--danger);\n      color: #ffffff;\n    }\n    .modal-backdrop {\n      position: fixed;\n      inset: 0;\n      background: rgba(0, 0, 0, 0.60);\n      backdrop-filter: blur(8px);\n      display: none;\n      align-items: center;\n      justify-content: center;\n      z-index: 120;\n      padding: 20px;\n    }\n    .modal-backdrop.open {\n      display: flex;\n    }\n    .modal-dialog {\n      width: 100%;\n      max-width: 460px;\n      background: var(--card-bg);\n      border: 1px solid var(--border-strong);\n      border-radius: var(--radius-lg);\n      box-shadow: var(--shadow-setup);\n      padding: 22px 24px 20px;\n      display: flex;\n      flex-direction: column;\n      gap: 14px;\n    }\n    .modal-title {\n      font-family: var(--font-display);\n      font-size: 18px;\n      font-weight: 600;\n      color: var(--text-primary);\n    }\n    .modal-body {\n      font-size: 13.5px;\n      line-height: 1.5;\n      color: var(--text-secondary);\n    }\n    .modal-actions {\n      display: flex;\n      align-items: center;\n      justify-content: flex-end;\n      gap: 10px;\n      margin-top: 4px;\n    }\n\n    .bottom-nav-row {\n      display: flex;\n      align-items: center;\n      justify-content: space-between;\n      padding-top: 14px;\n      margin-top: 6px;\n      border-top: 1px solid var(--divider);\n      gap: 12px;\n    }\n    .center-flash-row {\n      display: flex;\n      flex-direction: column;\n      align-items: center;\n      justify-content: center;\n      padding: 14px 0 4px;\n      gap: 12px;\n    }\n\n    .code-viewer {\n      background: var(--card-bg-secondary);\n      border: 1px solid var(--border-card);\n      border-radius: var(--radius);\n      padding: 14px;\n      font-family: var(--font-mono);\n      font-size: 12px;\n      line-height: 1.55;\n      color: var(--text-primary);\n      max-height: 300px;\n      overflow: auto;\n      white-space: pre;\n      user-select: text;\n    }\n\n    .toast-msg {\n      position: fixed;\n      bottom: 20px;\n      right: 24px;\n      padding: 10px 16px;\n      border-radius: var(--radius);\n      background: var(--card-bg);\n      border: 1px solid var(--border-strong);\n      box-shadow: var(--shadow-setup);\n      font-size: 13px;\n      font-weight: 500;\n      color: var(--text-primary);\n      z-index: 90;\n      opacity: 0;\n      transform: translateY(10px);\n      pointer-events: none;\n      transition: opacity 0.2s ease, transform 0.2s ease;\n    }\n    .toast-msg.visible {\n      opacity: 1;\n      transform: translateY(0);\n    }\n\n    /* Standard / Pro Segmented Mode Switch & Preset Cards */\n    .mode-segmented-switch {\n      display: inline-flex;\n      align-items: center;\n      background: var(--bg-input);\n      border: 1px solid var(--border-control);\n      border-radius: 999px;\n      padding: 2px;\n      gap: 2px;\n    }\n    .mode-seg-btn {\n      border: none;\n      background: transparent;\n      color: var(--text-secondary);\n      font-family: var(--font-sans);\n      font-size: 12px;\n      font-weight: 600;\n      padding: 4px 12px;\n      border-radius: 999px;\n      cursor: pointer;\n      transition: all 0.16s ease;\n    }\n    .mode-seg-btn.active-standard {\n      background: var(--accent);\n      color: #FFFFFF;\n    }\n    .mode-seg-btn.active-pro {\n      background: linear-gradient(135deg, #0A84FF 0%, #5E5CE6 100%);\n      color: #FFFFFF;\n    }\n    .presets-grid {\n      display: grid;\n      grid-template-columns: repeat(auto-fill, minmax(245px, 1fr));\n      gap: 10px;\n    }\n    .preset-card {\n      background: var(--bg-card);\n      border: 1px solid var(--border-subtle);\n      border-radius: var(--radius-md);\n      padding: 12px 14px;\n      cursor: pointer;\n      display: flex;\n      flex-direction: column;\n      gap: 6px;\n      transition: all 0.15s ease;\n    }\n    .preset-card:hover {\n      background: var(--bg-card-hover);\n      border-color: var(--border-strong);\n    }\n    .preset-card.selected {\n      border-color: var(--accent);\n      background: var(--accent-soft);\n    }\n    .admin-top-banner {\n      background: var(--warning-bg);\n      border-bottom: 1px solid var(--warning);\n      padding: 8px 20px;\n      display: flex;\n      align-items: center;\n      justify-content: space-between;\n      gap: 12px;\n      font-size: 12.5px;\n    }\n\n    .storage-input-group {\n      display: flex;\n      align-items: center;\n      gap: 8px;\n      flex-wrap: wrap;\n    }\n    .storage-num-input {\n      width: 115px;\n      background: var(--bg-input);\n      border: 1px solid var(--border-strong);\n      color: var(--text-primary);\n      border-radius: var(--radius-sm);\n      padding: 6px 10px;\n      font-size: 13.5px;\n      font-weight: 600;\n      font-family: var(--font-mono);\n      outline: none;\n    }\n    .storage-num-input:focus {\n      border-color: var(--accent);\n    }\n    .storage-num-input.invalid {\n      border-color: var(--danger);\n    }\n    .storage-presets-row {\n      display: flex;\n      align-items: center;\n      gap: 6px;\n      flex-wrap: wrap;\n      margin-top: 8px;\n    }\n    .storage-preset-pill {\n      border: 1px solid var(--border-subtle);\n      background: var(--bg-input);\n      color: var(--text-secondary);\n      border-radius: 999px;\n      padding: 3px 10px;\n      font-size: 11.5px;\n      font-weight: 500;\n      cursor: pointer;\n      transition: all 0.14s ease;\n    }\n    .storage-preset-pill:hover, .storage-preset-pill.active {\n      border-color: var(--accent);\n      background: var(--accent-soft);\n      color: var(--accent);\n    }\n\n    @media (max-width: 820px) {\n      .settings-shell {\n        flex-direction: column;\n      }\n      .nav-sidebar {\n        width: 100%;\n        flex-direction: row;\n        overflow-x: auto;\n        border-right: none;\n        border-bottom: 1px solid var(--divider);\n      }\n      .nav-list {\n        flex-direction: row;\n      }\n      .content-area {\n        padding: 18px 16px 32px;\n      }\n      .win-card {\n        flex-direction: column;\n        align-items: flex-start;\n      }\n      .card-right, .win-input, .win-combobox {\n        width: 100%;\n      }\n      .titlebar-left, .titlebar-right {\n        min-width: auto;\n      }\n      .appstore-grid {\n        grid-template-columns: 1fr;\n      }\n    }\n\n    @media (max-height: 760px) {\n      .window-titlebar {\n        height: 42px;\n        padding: 0 12px;\n      }\n      .setup-shell {\n        padding: 8px 12px;\n      }\n      .setup-top-tabs {\n        min-height: 42px;\n        padding: 4px 12px 0;\n      }\n      .setup-body {\n        padding: 12px 16px;\n      }\n      .setup-footer {\n        padding: 10px 16px;\n      }\n    }\n  \n    /* Lock and grey out all configuration controls while flashing/downloading is active */\n    body.is-flashing-locked .setup-tab-btn,\n    body.is-flashing-locked .nav-item,\n    body.is-flashing-locked .mode-seg-btn,\n    body.is-flashing-locked #btnToggleModeView,\n    body.is-flashing-locked #btnTitleBack,\n    body.is-flashing-locked #globalSearchInput,\n    body.is-flashing-locked #adminPrivilegeBanner,\n    body.is-flashing-locked .win-card:not(#flashPanelCard),\n    body.is-flashing-locked .expander-wrap,\n    body.is-flashing-locked .system-summary-banner,\n    body.is-flashing-locked .setup-footer-bar button:not([data-action=\"pause-resume-flash\"]):not([data-action=\"cancel-flash\"]),\n    body.is-flashing-locked .bottom-nav-row button:not([data-action=\"pause-resume-flash\"]):not([data-action=\"cancel-flash\"]) {\n      opacity: 0.45 !important;\n      pointer-events: none !important;\n      filter: grayscale(0.45);\n      user-select: none;\n    }";
  document.head.appendChild(st);
})();

    const state = {
      theme: "dark",
      ui_mode: "standard",
      preset_role: "custom",
      hostIsAdmin: true,
      hostOsName: "windows",
      viewMode: "setup",
      setupTab: "network",
      settingsTab: "system",
      searchQuery: "",
      openDropdownField: null,
      advancedExpanded: {
        network: false,
        apps: false,
        storage: false,
        power: false
      },
      configSavedLocally: false,
      savedConfigPath: "Instant_Server_Data/instant-server-config.json",

      // Q1: Identity, Network & Multi-Computer Cluster
      hostname: "instant-server",
      username: "admin",
      password: "",
      wifi_ssid: "",
      wifi_password: "",
      enable_usb_tethering: true,
      disable_wifi_power_save: true,
      enable_mosh_server: true,
      static_ip_cidr: "",
      static_gateway: "",
      static_dns: "",
      ssh_authorized_key: "",
      cluster_mode: "standalone",
      cluster_storage_policy: "mergerfs_union_safe",
      confirm_wipe_secondary: false,

      // Q2: Dashboards & Pre-Selected Self-Hosted Apps
      dashboard_choice: "both",
      selected_apps: ["jellyfin", "nextcloud", "pihole", "kiwix", "immich", "homeassistant", "vaultwarden"],
      appSearchQuery: "",
      appCategoryFilter: "All",
      custom_apps: [],
      customWebAppInput: "",
      prebundle_offline_images: false,
      enable_pihole_dns_prep: true,
      enable_samba_nas: true,
      enable_intel_vaapi: true,
      enable_tailscale: false,
      tailscale_authkey: "",
      tailscale_mode: "subnet_and_ssh",
      mc_server_type: "PAPER",
      mc_version: "26.3",
      mc_memory_heap: "4G",
      mc_use_aikar_flags: true,
      mc_gamemode: "survival",
      mc_difficulty: "normal",
      mc_online_mode: true,
      mc_max_players: 20,
      mc_view_distance: 10,
      mc_level_seed: "",
      mc_motd: "Instant Server Minecraft World",
      mc_ops_list: "",
      mc_whitelist: "",
      mc_modrinth_projects: "viaversion,chunky",
      mc_enable_rcon: true,
      mc_rcon_password: "instant-rcon-admin",
      mc_enable_backups: true,
      studio_app_id: "minecraft",
      studio_file_name: "server.properties",
      studio_new_file_name: "",
      app_overrides: {},

      // Q3: Target Computer Type, Age, RAM, ISO Source & USB Partitioning
      device_form_factor: "auto_detect",
      laptop_age_hint: "unknown",
      laptop_ram_hint: "unknown",
      usb_partition_scheme: "auto",
      iso_source_mode: "official_dl",
      custom_iso_path: "",
      confirm_usb_erase: true,

      install_mode: "dual_boot",
      dual_boot_Allocation_gb: 45,
      storageUnit: "GB",
      storageInputRaw: "45",
      storageInputError: "",
      confirm_erase_main_drive: false,
      confirm_wipe_server_data: false,
      enable_iphone_auto_backup: false,
      iphone_backup_mode: "",
      custom_apt_packages_raw: "",
      custom_config_path: "/etc/samba/smb-custom.conf",
      custom_config_body: "# Custom server configuration file\n",
      custom_config_files: {},
      ubuntu_release: "26.04",
      storage_layout: "direct",
      boot_mode: "auto",

      // Q4: Laptop Power & Survival
      ignore_lid_switch: true,
      screen_blank_seconds: "60",
      battery_charge_start_pct: 40,
      battery_charge_stop_pct: 60,
      ups_shutdown_battery_pct: 15,
      enable_low_ram_zram: true,
      safe_gpu_nomodeset: false,

      // Custom Server Homepage Dashboard, Offline Wikipedia & Pro Mode Controls
      enable_custom_dashboard: true,
      dashboard_port: 8088,
      enable_auto_updates: true,
      auto_update_window: "03:00",
      enable_scheduled_backups: true,
      backup_retention_days: 14,
      preselect_kiwix_zim: "wikipedia_en_simple",
      ssh_port: 22,
      enable_ufw_firewall: true,
      enable_fail2ban: true,
      enable_ipv4_forwarding: true,
      enable_tcp_bbr: true,
      vm_swappiness: 10,
      inotify_max_watches: 524288,
      root_filesystem: "ext4",
      enable_ssd_noatime_trim: true,
      enable_smartd_monitor: true,
      zram_algorithm: "zstd",
      zram_percent: 50,
      cpu_governor: "schedutil",
      disable_cpu_turbo_boost: false,
      docker_subnet_cidr: "172.17.0.1/16",
      enable_prometheus_metrics: false,
      custom_kernel_args: "",
      custom_post_install_script: "",

      // Connected USB Devices & Flashing Status
      usbDevices: [],
      selectedUsbId: "",
      scanningUsb: false,
      resettingUsb: false,

      flashing: false,
      flashPaused: false,
      flashCancelled: false,
      flashOpSeq: 0,
      cacheRetentionMode: "keep",
      cacheStatusMessage: "Keep cached ISO & app files (Default) — If you close the app or reflash later with new apps/edits, only the new/changed apps are installed.",
      appsExpanded: false,
      flashProgress: 0,
      flashStatusText: "",
      flashEtaText: "",
      flashLogs: [],
      renderedLogCount: 0,
      activeProgressLine: "",
      logAutoScroll: true,
      closeWarningOpen: false,
      forceClosing: false,
      validationErrors: [],

      lanScanResults: [],
      lanScanning: false
    };

    let pollTimer = null;

    function esc(val) {
      return String(val ?? "")
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;");
    }

    function showToast(msg) {
      const el = document.getElementById("toastMsg");
      if (!el) return;
      el.textContent = msg;
      el.classList.add("visible");
      clearTimeout(el._timer);
      el._timer = setTimeout(() => el.classList.remove("visible"), 2800);
    }

    async function callBackend(endpoint, method = "GET", payload = null) {
      try {
        if (window.go && window.go.main && window.go.main.App) {
          const wailsApp = window.go.main.App;
          if (endpoint === "host" && wailsApp.ProbeHost) return await wailsApp.ProbeHost();
          if (endpoint === "fit-window" && wailsApp.FitWindowToScreen && payload) return await wailsApp.FitWindowToScreen(payload.avail_width || 1280, payload.avail_height || 720);
          if (endpoint === "toggle-maximize" && wailsApp.ToggleMaximizeWindow) return await wailsApp.ToggleMaximizeWindow();
          if (endpoint === "elevate" && wailsApp.ElevateAdmin) return await wailsApp.ElevateAdmin();
          if (endpoint === "usb" && wailsApp.ScanUSBDevices) return await wailsApp.ScanUSBDevices();
          if (endpoint === "reset-usb" && wailsApp.ResetUSBDrive) return await wailsApp.ResetUSBDrive((payload && payload.device) || "");
          if (endpoint === "load-config" && wailsApp.LoadConfig) return await wailsApp.LoadConfig();
          if (endpoint === "save-config" && wailsApp.SaveConfig) return await wailsApp.SaveConfig(payload);
          if (endpoint === "custom-app" && wailsApp.AddCustomApp) return await wailsApp.AddCustomApp(payload);
          if (endpoint === "flash" && wailsApp.FlashUSBDevice) return await wailsApp.FlashUSBDevice(payload, (payload && payload.custom_iso_path) || "");
          if (endpoint === "pause" && wailsApp.PauseFlashing) return await wailsApp.PauseFlashing();
          if (endpoint === "resume" && wailsApp.ResumeFlashing) return await wailsApp.ResumeFlashing();
          if (endpoint === "cancel" && wailsApp.CancelFlashing) return await wailsApp.CancelFlashing();
          if (endpoint === "pause-and-exit" && wailsApp.PauseAndPrepareExit) return await wailsApp.PauseAndPrepareExit();
          if (endpoint === "cache" && wailsApp.ManageFlashCache) return await wailsApp.ManageFlashCache((payload && payload.action) || "keep");
          if (endpoint === "status" && wailsApp.PollBuildStatus) return await wailsApp.PollBuildStatus();
          if (endpoint === "discover" && wailsApp.DiscoverServers) return await wailsApp.DiscoverServers(state.hostname);
        }
        if (window.location.protocol.startsWith("http")) {
          const targetUrl = window.location.origin + "/" + "api/" + endpoint;
          const opts = {
            method,
            headers: { "Content-Type": "application/json" },
            cache: "no-store"
          };
          if (payload && method === "POST") {
            opts.body = JSON.stringify(payload);
          }
          const resp = await fetch(targetUrl, opts);
          if (resp.ok) {
            return await resp.json();
          }
        }
        } catch (_err) {}
      return null;
    }

    function serializeProfilePayload() {
      const selectedDev = state.usbDevices.find(d => d.device_id === state.selectedUsbId);
      return {
        hostname: (state.hostname || "instant-server").trim(),
        username: (state.username || "admin").trim(),
        password: state.password,
        wifi_ssid: state.wifi_ssid.trim(),
        wifi_password: state.wifi_password,
        enable_usb_tethering: state.enable_usb_tethering,
        disable_wifi_power_save: state.disable_wifi_power_save,
        enable_mosh_server: state.enable_mosh_server,
        static_ip_cidr: state.static_ip_cidr.trim(),
        static_gateway: state.static_gateway.trim(),
        static_dns: state.static_dns ? state.static_dns.split(",").map(s => s.trim()).filter(Boolean) : [],
        ssh_authorized_keys: state.ssh_authorized_key.trim() ? [state.ssh_authorized_key.trim()] : [],
        cluster_mode: state.cluster_mode,
        cluster_storage_policy: state.cluster_storage_policy,
        cluster_confirm_wipe_secondary: state.confirm_wipe_secondary ? "WIPE-SECONDARY" : "",
        device_form_factor: state.device_form_factor,
        dashboard_choice: state.dashboard_choice,
        selected_apps: Array.isArray(state.selected_apps) ? state.selected_apps : [],
        custom_apps: Array.isArray(state.custom_apps) ? state.custom_apps : [],
        app_overrides: state.app_overrides || {},
        minecraft_config: {
          server_type: state.mc_server_type || "PAPER",
          version: state.mc_version || "26.3",
          memory_heap: state.mc_memory_heap || "4G",
          use_aikar_flags: !!state.mc_use_aikar_flags,
          gamemode: state.mc_gamemode || "survival",
          difficulty: state.mc_difficulty || "normal",
          online_mode: !!state.mc_online_mode,
          max_players: Number(state.mc_max_players) || 20,
          view_distance: Number(state.mc_view_distance) || 10,
          simulation_distance: 10,
          level_seed: (state.mc_level_seed || "").trim(),
          motd: (state.mc_motd || "Instant Server Minecraft World").trim(),
          ops_list: (state.mc_ops_list || "").trim(),
          whitelist: (state.mc_whitelist || "").trim(),
          modrinth_projects: (state.mc_modrinth_projects || "").trim(),
          enable_rcon: !!state.mc_enable_rcon,
          rcon_password: (state.mc_rcon_password || "instant-rcon-admin").trim(),
          enable_backups: !!state.mc_enable_backups
        },
        prebundle_offline_images: state.prebundle_offline_images,
        enable_pihole_dns_prep: state.enable_pihole_dns_prep,
        enable_samba_nas: state.enable_samba_nas,
        enable_intel_vaapi: state.enable_intel_vaapi,
        enable_tailscale: state.enable_tailscale,
        tailscale_authkey: state.tailscale_authkey.trim(),
        tailscale_mode: state.tailscale_mode || "subnet_and_ssh",
        laptop_age_hint: state.laptop_age_hint,
        laptop_ram_hint: state.laptop_ram_hint,
        usb_partition_scheme: state.usb_partition_scheme,
        custom_iso_path: state.iso_source_mode === "local_file" ? state.custom_iso_path.trim() : "",
        confirm_usb_erase: state.confirm_usb_erase,
        format_usb: true,
        install_mode: state.install_mode,
        dual_boot_Allocation_gb: Number(state.dual_boot_Allocation_gb) || 45,
        confirm_erase_main_drive: state.confirm_erase_main_drive,
        confirm_wipe_server_data: state.confirm_wipe_server_data ? "WIPE-SERVER-DATA" : "",
        enable_iphone_auto_backup: !!state.enable_iphone_auto_backup,
        iphone_backup_mode: state.iphone_backup_mode || "all_three",
        custom_config_files: state.custom_config_files || {},
        custom_apt_packages: (state.custom_apt_packages_raw || "").split(/[\s,]+/).map(s => s.trim()).filter(Boolean),
        ubuntu_release: String(state.ubuntu_release),
        storage_layout: state.storage_layout,
        boot_mode: state.boot_mode,
        ignore_lid_switch: state.ignore_lid_switch,
        screen_blank_seconds: Number(state.screen_blank_seconds) || 60,
        battery_charge_start_pct: Number(state.battery_charge_start_pct) || 40,
        battery_charge_stop_pct: Number(state.battery_charge_stop_pct) || 60,
        ups_shutdown_battery_pct: Number(state.ups_shutdown_battery_pct) || 15,
        enable_low_ram_zram: state.enable_low_ram_zram,
        safe_gpu_nomodeset: state.safe_gpu_nomodeset,
        ui_mode: state.ui_mode,
        preset_role: state.preset_role,
        enable_custom_dashboard: state.enable_custom_dashboard,
        dashboard_port: Number(state.dashboard_port) || 8088,
        enable_auto_updates: state.enable_auto_updates,
        auto_update_window: state.auto_update_window,
        enable_scheduled_backups: state.enable_scheduled_backups,
        backup_retention_days: Number(state.backup_retention_days) || 14,
        preselect_kiwix_zims: [state.preselect_kiwix_zim || "wikipedia_en_simple"],
        ssh_port: Number(state.ssh_port) || 22,
        enable_ufw_firewall: state.enable_ufw_firewall,
        enable_fail2ban: state.enable_fail2ban,
        enable_ipv4_forwarding: state.enable_ipv4_forwarding,
        enable_tcp_bbr: state.enable_tcp_bbr,
        vm_swappiness: Number(state.vm_swappiness) || 10,
        inotify_max_watches: Number(state.inotify_max_watches) || 524288,
        root_filesystem: state.root_filesystem,
        enable_ssd_noatime_trim: state.enable_ssd_noatime_trim,
        enable_smartd_monitor: state.enable_smartd_monitor,
        zram_algorithm: state.zram_algorithm,
        zram_percent: Number(state.zram_percent) || 50,
        cpu_governor: state.cpu_governor,
        disable_cpu_turbo_boost: state.disable_cpu_turbo_boost,
        docker_subnet_cidr: state.docker_subnet_cidr.trim() || "172.17.0.1/16",
        enable_prometheus_metrics: state.enable_prometheus_metrics,
        custom_kernel_args: state.custom_kernel_args.trim(),
        custom_post_install_script: state.custom_post_install_script,
        selected_usb_device: selectedDev ? selectedDev.device_node : state.selectedUsbId
      };
    }

    function applyConfigToState(cfg) {
      if (!cfg || typeof cfg !== "object") return;
      const keys = [
        "hostname", "username", "password", "wifi_ssid", "wifi_password",
        "enable_usb_tethering", "static_ip_cidr", "static_gateway",
        "cluster_mode", "cluster_storage_policy", "device_form_factor",
        "dashboard_choice", "prebundle_offline_images", "enable_pihole_dns_prep",
        "enable_samba_nas", "enable_intel_vaapi", "enable_tailscale", "tailscale_authkey", "tailscale_mode",
        "laptop_age_hint", "laptop_ram_hint", "usb_partition_scheme", "custom_iso_path",
        "confirm_usb_erase", "install_mode", "dual_boot_Allocation_gb", "confirm_erase_main_drive",
        "ubuntu_release", "storage_layout", "boot_mode", "ignore_lid_switch",
        "screen_blank_seconds", "battery_charge_start_pct", "battery_charge_stop_pct",
        "ups_shutdown_battery_pct", "enable_low_ram_zram", "safe_gpu_nomodeset",
        "ui_mode", "preset_role", "enable_custom_dashboard", "dashboard_port",
        "enable_auto_updates", "auto_update_window", "enable_scheduled_backups",
        "backup_retention_days", "ssh_port", "enable_ufw_firewall", "enable_fail2ban",
        "enable_ipv4_forwarding", "enable_tcp_bbr", "vm_swappiness", "inotify_max_watches",
        "root_filesystem", "enable_ssd_noatime_trim", "enable_smartd_monitor",
        "zram_algorithm", "zram_percent", "cpu_governor", "disable_cpu_turbo_boost",
        "docker_subnet_cidr", "enable_prometheus_metrics", "custom_kernel_args",
        "custom_post_install_script"
      ];
      keys.forEach(k => {
        if (cfg[k] !== undefined) state[k] = cfg[k];
      });
      if (Array.isArray(cfg.selected_apps)) {
        state.selected_apps = cfg.selected_apps;
      }
      if (Array.isArray(cfg.custom_apps)) {
        state.custom_apps = cfg.custom_apps;
        cfg.custom_apps.forEach(ca => {
          if (ca && ca.id && !APP_STORE_CATALOG.some(a => a.id === ca.id)) {
            APP_STORE_CATALOG.unshift({
              id: ca.id,
              title: ca.title || ca.id,
              category: ca.category || "Security & Tools",
              recommended: true,
              tag: "Custom • Port " + (ca.port || 8080),
              ram: "128 MB",
              hardware_tier: "old_friendly",
              image: ca.image || "",
              port: ca.port || 8080,
              desc: ca.description || ("Custom container (" + (ca.image || ca.id) + ")")
            });
          }
        });
      }
      if (cfg.custom_iso_path && String(cfg.custom_iso_path).trim()) {
        state.iso_source_mode = "local_file";
      }
      state.screen_blank_seconds = String(state.screen_blank_seconds || "60");
      state.ubuntu_release = String(state.ubuntu_release || "26.04");
      if (Array.isArray(cfg.static_dns)) {
        state.static_dns = cfg.static_dns.join(", ");
      }
      if (Array.isArray(cfg.ssh_authorized_keys) && cfg.ssh_authorized_keys.length > 0) {
        state.ssh_authorized_key = cfg.ssh_authorized_keys[0];
      }
    }

    async function initAppStartup() {
      try {
        await callBackend("fit-window", "POST", {
          avail_width: Number(window.screen && window.screen.availWidth) || 1280,
          avail_height: Number(window.screen && window.screen.availHeight) || 720
        });
      } catch (_e) {}
      let loadedFromDisk = false;

      if (window.__INSTANT_SERVER_BOOT_CONFIG__ && window.__INSTANT_SERVER_BOOT_CONFIG__.exists && window.__INSTANT_SERVER_BOOT_CONFIG__.config) {
        applyConfigToState(window.__INSTANT_SERVER_BOOT_CONFIG__.config);
        state.configSavedLocally = true;
        state.savedConfigPath = window.__INSTANT_SERVER_BOOT_CONFIG__.path || "Instant_Server_Data/instant-server-config.json";
        state.viewMode = "settings";
        loadedFromDisk = true;
      }

      if (!loadedFromDisk) {
        const res = await callBackend("load-config", "GET");
        if (res && res.exists && res.config) {
          applyConfigToState(res.config);
          state.configSavedLocally = true;
          state.savedConfigPath = res.path || "Instant_Server_Data/instant-server-config.json";
          state.viewMode = "settings";
          loadedFromDisk = true;
        }
      }

      if (!loadedFromDisk) {
        try {
          const cached = localStorage.getItem("instant_server_saved_config");
          if (cached) {
            applyConfigToState(JSON.parse(cached));
            state.configSavedLocally = true;
            state.viewMode = "settings";
          }
        } catch (_e) {}
      }

      const hostInfo = await callBackend("host", "GET");
      if (hostInfo && typeof hostInfo === "object") {
        state.hostIsAdmin = !!hostInfo.is_admin_or_root;
        state.hostOsName = String(hostInfo.host_os || "windows");
        const adminBanner = document.getElementById("adminPrivilegeBanner");
        if (adminBanner) {
          adminBanner.style.display = (!state.hostIsAdmin && state.hostOsName === "windows") ? "flex" : "none";
        }
      }
      await scanUsbDrives(true);
      shouldAnimateTab = true;
      renderApp();
    }

    
    async function resetSelectedUsbDrive() {
      if (state.flashing || state.resettingUsb) return;
      const selectedDev = state.usbDevices.find(d => d.device_id === state.selectedUsbId && d.triple_lock_passed)
        || state.usbDevices.find(d => d.triple_lock_passed);
      const targetNode = selectedDev ? selectedDev.device_node : state.selectedUsbId;
      if (!targetNode || targetNode === "usb_sim_none") {
        showToast("Plug in and select a removable USB drive first");
        return;
      }
      state.resettingUsb = true;
      renderApp();
      showToast("Clearing write-protection and formatting " + targetNode + " as FAT32...");
      const res = await callBackend("reset-usb", "POST", { device: targetNode });
      state.resettingUsb = false;
      if (res && res.ok) {
        showToast(res.message || ("Reset " + targetNode + " to a normal FAT32 drive"));
        await scanUsbDrives(true);
      } else if (res && res.elevated_relaunch) {
        showToast(res.error || "Relaunching as Administrator...");
      } else {
        showToast((res && res.error) ? res.error : ("Failed to reset " + targetNode));
      }
      renderApp();
    }

    async function scanUsbDrives(silent = false) {
      state.scanningUsb = true;
      if (!silent) renderApp();

      const liveList = await callBackend("usb", "GET");
      if (Array.isArray(liveList) && liveList.length > 0) {
        state.usbDevices = liveList;
      } else if (state.usbDevices.length === 0) {
        state.usbDevices = [
          {
            device_id: "usb0",
            device_node: "/dev/sdb",
            model: "SanDisk Ultra USB 3.0",
            size_gb: 32.0,
            bus: "usb",
            removable: true,
            mountpoints: ["D:\\"],
            triple_lock_passed: true,
            block_reasons: []
          },
          {
            device_id: "sys0",
            device_node: "System Disk 0",
            model: "Internal OS Drive",
            size_gb: 512.0,
            bus: "nvme",
            removable: false,
            mountpoints: ["C:\\"],
            triple_lock_passed: false,
            block_reasons: ["System OS drive is protected"]
          }
        ];
      }

      const firstValid = state.usbDevices.find(d => d.triple_lock_passed);
      if (firstValid && !state.selectedUsbId) {
        state.selectedUsbId = firstValid.device_id;
      }
      state.scanningUsb = false;
      if (!silent) {
        showToast("USB drives refreshed");
        renderApp();
      }
    }

    async function saveConfigurationAndOpenHome() {
      const payload = serializeProfilePayload();
      const res = await callBackend("save-config", "POST", payload);
      try {
        localStorage.setItem("instant_server_saved_config", JSON.stringify(payload));
      } catch (_e) {}
      state.configSavedLocally = true;
      if (res && res.path) {
        state.savedConfigPath = res.path;
      }
      if (state.viewMode !== "settings") {
        shouldAnimateTab = true;
      }
      state.viewMode = "settings";
      state.settingsTab = "system";
      showToast("Saved to Instant_Server_Data/instant-server-config.json");
      renderApp();
    }

    function checkIncompleteRequirements() {
      const missing = [];
      if (!state.hostname || !/^[a-z0-9][a-z0-9-]{0,61}$/i.test(state.hostname.trim())) {
        missing.push("Enter a valid Server Name (letters, numbers, hyphens).");
      }
      if (!state.username || !/^[a-z_][a-z0-9_-]{0,31}$/i.test(state.username.trim())) {
        missing.push("Enter a valid Admin Username.");
      }
      if (!state.password && !state.ssh_authorized_key.trim()) {
        missing.push("Set an Admin Password (or add an SSH key in Network advanced options).");
      }
      if (state.wifi_ssid.trim() && state.wifi_password.length > 0 && state.wifi_password.length < 8) {
        missing.push("Wi-Fi password must be at least 8 characters.");
      }
      if (state.install_mode === "dual_boot") {
        const gbVal = Number(state.dual_boot_Allocation_gb);
        if (!Number.isFinite(gbVal) || gbVal < 15 || gbVal > 65536 || state.storageInputError) {
          missing.push(state.storageInputError || "Enter a valid Server Partition Size between 15 GB and 64 TB (65,536 GB).");
        }
      }
      if (state.install_mode === "auto_erase" && !state.confirm_erase_main_drive) {
        missing.push("Confirm 'I know what I'm doing' for Erase Main Drive mode, or switch to Dual-Boot.");
      }
      if (state.install_mode === "reinstall_wipe_data" && !state.confirm_wipe_server_data) {
        missing.push("Confirm wiping all existing server data for Factory Reinstall mode, or switch to Reinstall While Retaining Data.");
      }
      if (state.cluster_mode !== "standalone" && state.cluster_storage_policy === "wipe_secondary_expand" && !state.confirm_wipe_secondary) {
        missing.push("Confirm erasing the second computer's files, or switch to 'Keep all files on both computers'.");
      }
      if (state.iso_source_mode === "local_file" && !state.custom_iso_path.trim()) {
        missing.push("Enter the file path to your local Ubuntu Server .iso file, or switch to Automatic Official ISO Download.");
      }
      const usb = state.usbDevices.find(d => d.device_id === state.selectedUsbId && d.triple_lock_passed);
      if (!usb) {
        missing.push("Select an approved removable USB flash drive in Laptop & USB.");
      }
      if (!state.confirm_usb_erase) {
        missing.push("Confirm that all existing data on the selected USB drive will be erased and formatted.");
      }
      return missing;
    }

    function buildAsciiTerminalBar(pct, label, detail) {
      const clamped = Math.max(0, Math.min(100, Number(pct) || 0));
      const width = 22;
      const filled = Math.round((clamped / 100) * width);
      const bar = "█".repeat(filled) + "░".repeat(Math.max(0, width - filled));
      const pctStr = clamped.toFixed(1).padStart(5, " ") + "%";
      return detail
        ? `⟳ ${label}: [${bar}] ${pctStr} │ ${detail}`
        : `⟳ ${label}: [${bar}] ${pctStr}`;
    }

    function scrollTerminalToBottomIfSticky() {
      const logBox = document.getElementById("flashLogBox");
      if (!logBox) return;
      if (state.logAutoScroll) {
        logBox._programmaticScroll = true;
        logBox.scrollTop = logBox.scrollHeight;
        setTimeout(() => { logBox._programmaticScroll = false; }, 40);
      }
      const resumeBtn = document.getElementById("btnResumeAutoScroll");
      if (resumeBtn) {
        resumeBtn.style.display = state.logAutoScroll ? "none" : "inline-flex";
      }
    }

    function syncFlashingLockState() {
      document.body.classList.toggle("is-flashing-locked", !!state.flashing);
      const lockBadge = document.getElementById("flashConfigLockedBadge");
      if (lockBadge) {
        lockBadge.style.display = state.flashing ? "inline-flex" : "none";
      }
    }

    function updateFlashPanelInPlace() {
      syncFlashingLockState();
      const panel = document.getElementById("flashPanelCard");
      if (!panel) {
        renderApp();
        return;
      }

      const titleEl = document.getElementById("flashStatusTitle");
      const nextTitle = state.flashStatusText || "Preparing bootable USB...";
      if (titleEl && titleEl.textContent !== nextTitle) {
        titleEl.textContent = nextTitle;
      }

      const badgeEl = document.getElementById("flashBadgePct");
      if (badgeEl) {
        const nextPctTxt = `${state.flashProgress}%`;
        if (badgeEl.textContent !== nextPctTxt) badgeEl.textContent = nextPctTxt;
        const nextBadgeCls = "badge " + (
          state.flashProgress === 100
            ? "badge-success"
            : state.flashCancelled
            ? "badge-warning"
            : "badge-accent"
        );
        if (badgeEl.className !== nextBadgeCls) badgeEl.className = nextBadgeCls;
      }
      const etaEl = document.getElementById("flashEtaBadge");
      if (etaEl) {
        if (state.flashEtaText && state.flashing) {
          if (etaEl.style.display !== "inline-flex") etaEl.style.display = "inline-flex";
          if (etaEl.textContent !== state.flashEtaText) etaEl.textContent = state.flashEtaText;
        } else if (etaEl.style.display !== "none") {
          etaEl.style.display = "none";
          etaEl.textContent = "";
        }
      }

      const fillEl = document.getElementById("flashProgressFill");
      if (fillEl) {
        const nextWidth = `${state.flashProgress}%`;
        if (fillEl.style.width !== nextWidth) fillEl.style.width = nextWidth;
      }

      const hdrActions = document.getElementById("flashPanelHeaderActions");
      if (hdrActions) {
        if (state.flashing) {
          const pauseLbl = state.flashPaused ? `Resume (${state.flashProgress}%)` : "Pause";
          const pauseCls = state.flashPaused ? "win-btn win-btn-sm win-btn-accent" : "win-btn win-btn-sm";
          let pauseBtn = hdrActions.querySelector("[data-action='pause-resume-flash']");
          if (!pauseBtn) {
            hdrActions.innerHTML = `
              <button type="button" class="${pauseCls}" data-action="pause-resume-flash">${pauseLbl}</button>
              <button type="button" class="win-btn win-btn-sm win-btn-danger" data-action="cancel-flash">Cancel</button>
            `;
          } else {
            if (pauseBtn.textContent !== pauseLbl) pauseBtn.textContent = pauseLbl;
            if (pauseBtn.className !== pauseCls) pauseBtn.className = pauseCls;
          }
        } else if (hdrActions.innerHTML !== "") {
          hdrActions.innerHTML = "";
        }
      }

      const stageWrap = document.getElementById("flashStageStepsRow");
      if (stageWrap) {
        const nextStageHtml = renderFlashPipelineStagesHtml();
        if (stageWrap._lastHtml !== nextStageHtml) {
          stageWrap._lastHtml = nextStageHtml;
          stageWrap.innerHTML = nextStageHtml;
        }
      }

      document.querySelectorAll("[data-action='start-flash']").forEach(btn => {
        btn.disabled = state.flashing;
        btn.textContent = state.flashing
          ? (state.flashPaused ? `Paused at ${state.flashProgress}%` : "Flashing Bootable USB...")
          : "Confirm & Flash Bootable USB";
      });
      document.querySelectorAll("[data-action='pause-resume-flash']").forEach(btn => {
        btn.textContent = state.flashPaused ? `Resume (${state.flashProgress}%)` : "Pause";
      });
      document.querySelectorAll(".footer-cancel-flash-wrap").forEach(wrap => {
        wrap.style.display = state.flashing ? "inline-flex" : "none";
      });

      const linesContainer = document.getElementById("flashLogLines");
      if (linesContainer) {
        const existingNodes = linesContainer.children;
        while (existingNodes.length > state.flashLogs.length) {
          linesContainer.removeChild(linesContainer.lastChild);
        }
        for (let i = 0; i < state.flashLogs.length; i++) {
          if (i < existingNodes.length) {
            if (existingNodes[i].textContent !== state.flashLogs[i]) {
              existingNodes[i].textContent = state.flashLogs[i];
            }
          } else {
            const row = document.createElement("div");
            row.className = "term-log-line";
            row.textContent = state.flashLogs[i];
            linesContainer.appendChild(row);
          }
        }
        state.renderedLogCount = state.flashLogs.length;
      }

      const liveRow = document.getElementById("flashTerminalLiveRow");
      if (liveRow) {
        if (state.activeProgressLine) {
          if (liveRow.style.display !== "block") liveRow.style.display = "block";
          if (liveRow.textContent !== state.activeProgressLine) liveRow.textContent = state.activeProgressLine;
        } else if (liveRow.style.display !== "none") {
          liveRow.style.display = "none";
          liveRow.textContent = "";
        }
      }

      scrollTerminalToBottomIfSticky();
    }

    function renderFlashPipelineStagesHtml() {
      const p = Number(state.flashProgress) || 0;
      const stages = [
        { label: "1. Config & Checks", min: 0, doneAt: 10 },
        { label: "2. Ubuntu ISO", min: 10, doneAt: 42 },
        { label: "3. App Packages", min: 42, doneAt: 55 },
        { label: "4. USB Write", min: 55, doneAt: 92 },
        { label: "5. Verify & Ready", min: 92, doneAt: 100 }
      ];
      return stages.map(st => {
        const isDone = p >= st.doneAt;
        const isActive = !isDone && p >= st.min && state.flashing;
        const badgeCls = isDone ? "badge-success" : (isActive ? (state.flashPaused ? "badge-warning" : "badge-accent") : "");
        const prefix = isDone ? " " : (isActive ? (state.flashPaused ? "⏸ " : "⟳ ") : "");
        return `<span class="badge ${badgeCls}" style="font-size:11px;padding:3px 8px;">${prefix}${esc(st.label)}</span>`;
      }).join("");
    }

    async function togglePauseResumeFlashing() {
      if (!state.flashing && !state.flashPaused) return;
      if (state.pauseResumeBusy) return;
      state.pauseResumeBusy = true;
      state.flashOpSeq = (state.flashOpSeq || 0) + 1;
      try {
        if (!state.flashPaused) {
          state.flashPaused = true;
          state.activeProgressLine = "⏸ PAUSED │ Progress saved as .part";
          state.flashStatusText = `Paused at ${state.flashProgress}% (saved as .part). Click Resume to continue.`;
          state.flashLogs.push(`[PAUSED] Paused at ${state.flashProgress}% (saved as .part).`);
          updateFlashPanelInPlace();
          showToast("Paused (saved as .part)");
          await callBackend("pause", "POST");
        } else {
          state.flashPaused = false;
          state.flashing = true;
          state.lastResumeEpoch = Date.now();
          state.activeProgressLine = `⟳ Resuming from ${state.flashProgress}% (.part)...`;
          state.flashStatusText = `Resuming from ${state.flashProgress}% (saved as .part)...`;
          state.flashLogs.push(`[RESUMED] Continuing from ${state.flashProgress}% (.part).`);
          updateFlashPanelInPlace();
          showToast("Resumed from .part");
          await callBackend("resume", "POST");
          state.flashOpSeq = (state.flashOpSeq || 0) + 1;
        }
      } finally {
        state.pauseResumeBusy = false;
        updateFlashPanelInPlace();
      }
    }

    async function addCustomWebAppFromInput() {
      const raw = (state.customWebAppInput || "").trim();
      if (!raw) {
        showToast("Paste a Docker image, docker run command, or GitHub/Docker Hub URL");
        return;
      }
      let parsed = null;
      const res = await callBackend("custom-app", "POST", { raw_input: raw });
      if (res && res.ok && res.app) {
        parsed = res.app;
      } else {
        let image = raw;
        let port = 8080;
        const mPort = raw.match(/-p\s+(\d{2,5}):(\d{2,5})/) || raw.match(/:(\d{2,5})\b/);
        if (mPort) port = Number(mPort[1]);
        if (raw.includes("hub.docker.com/r/")) {
          image = raw.split("hub.docker.com/r/")[1].split(/[/?#]/).slice(0, 2).join("/");
        } else if (raw.includes("github.com/")) {
          const parts = raw.split("github.com/")[1].split(/[/?#]/).slice(0, 2);
          image = "ghcr.io/" + parts.join("/").toLowerCase() + ":latest";
        } else if (raw.startsWith("docker run")) {
          const tokens = raw.split(/\s+/);
          image = tokens[tokens.length - 1];
        }
        if (!image.includes(":")) image += ":latest";
        const baseName = image.split("/").pop().split(":")[0].replace(/[^a-z0-9_-]/gi, "-").toLowerCase() || "custom-app";
        parsed = {
          id: "custom_" + baseName.replace(/-/g, "_"),
          title: baseName.replace(/[-_]/g, " ").replace(/\b\w/g, c => c.toUpperCase()),
          category: "Security & Tools",
          image,
          port,
          volumes: [`/DATA/AppData/${baseName}:/data`],
          description: `Custom container (${image}) on port ${port}.`
        };
      }
      if (!state.custom_apps.some(a => a.id === parsed.id)) {
        state.custom_apps.push(parsed);
      }
      if (!APP_STORE_CATALOG.some(a => a.id === parsed.id)) {
        APP_STORE_CATALOG.unshift({
          id: parsed.id,
          title: parsed.title,
          category: "Security & Tools",
          recommended: true,
          tag: `Custom • Port ${parsed.port}`,
          ram: "128 MB",
          hardware_tier: "old_friendly",
          image: parsed.image,
          port: parsed.port,
          desc: parsed.description
        });
      }
      if (!state.selected_apps.includes(parsed.id)) {
        state.selected_apps.push(parsed.id);
      }
      state.customWebAppInput = "";
      showToast(`Added & selected ${parsed.title} (${parsed.image})`);
      renderApp();
    }

    async function cancelFlashingProcess(exitAfterCancel = false) {
      state.flashOpSeq = (state.flashOpSeq || 0) + 1;
      clearInterval(pollTimer);
      pollTimer = null;
      state.flashing = false;
      state.flashPaused = false;
      state.flashCancelled = true;
      state.activeProgressLine = "";
      state.flashEtaText = "";
      state.flashStatusText = "Flashing cancelled by user — cached ISO & app files are kept so you can resume or retry anytime.";
      state.flashLogs.push("[CANCELLED] Flashing stopped by user. Downloaded ISO & staged app files are preserved.");
      updateFlashPanelInPlace();
      renderApp();
      showToast("Flashing cancelled — cached files preserved");
      await callBackend("cancel", "POST");
      if (exitAfterCancel) {
        state.forceClosing = true;
        await callBackend("shutdown", "POST", { force: true });
        window.close();
      }
    }

    async function pauseAndExitFlashingProcess() {
      state.flashOpSeq = (state.flashOpSeq || 0) + 1;
      clearInterval(pollTimer);
      pollTimer = null;
      state.flashPaused = true;
      state.flashing = false;
      state.activeProgressLine = "⏸ PAUSED │ Progress saved as .part";
      state.flashStatusText = `Paused at ${state.flashProgress}% (saved as .part).`;
      state.flashLogs.push(`[PAUSED] Progress at ${state.flashProgress}% saved as .part.`);
      updateFlashPanelInPlace();
      showToast("Progress & downloaded files saved — exiting safely");
      await callBackend("pause-and-exit", "POST", {});
      state.forceClosing = true;
      window.close();
    }

    async function handleCacheRetentionAction(mode) {
      state.cacheRetentionMode = mode;
      const res = await callBackend("cache", "POST", { action: mode });
      if (mode === "delete") {
        state.cacheStatusMessage = (res && res.message)
          ? res.message
          : "Deleted cached ISO and staging files.";
        showToast("Deleted cached files");
      } else {
        const sizeInfo = (res && res.total_mb && res.total_mb !== "0.0 MB") ? ` (${res.total_mb})` : "";
        state.cacheStatusMessage = `Keeping cached files${sizeInfo} (Default).`;
        showToast("Keeping cached files");
      }
      renderApp();
    }

    async function startFlashingProcess() {
      if (state.flashing) return;
      const missing = checkIncompleteRequirements();
      state.validationErrors = missing;
      if (missing.length > 0) {
        showToast("Complete required settings before flashing");
        renderApp();
        return;
      }

      const payload = serializeProfilePayload();
      try {
        localStorage.setItem("instant_server_saved_config", JSON.stringify(payload));
      } catch (_e) {}

      try {
        if (navigator.wakeLock && navigator.wakeLock.request) {
          navigator.wakeLock.request("screen").catch(() => {});
        }
      } catch (_wl) {}
      state.flashing = true;
      state.flashPaused = false;
      state.flashCancelled = false;
      state.flashProgress = 6;
      state.flashEtaText = "ETA: calculating...";
      state.logAutoScroll = true;
      state.renderedLogCount = 0;
      const flashStartEpoch = Date.now();
      const spinFrames = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];
      let spinIdx = 0;
      state.flashStatusText = `Preparing Ubuntu Server ${state.ubuntu_release} LTS bootable USB...`;
      state.flashLogs = [
        "[00:00] Saved config to Instant_Server_Data/instant-server-config.json"
      ];
      state.activeProgressLine = buildAsciiTerminalBar(6.0, "Initializing Pipeline", "Checking local ISO cache & Ubuntu mirrors...");
      renderApp();
      state.renderedLogCount = state.flashLogs.length;
      scrollTerminalToBottomIfSticky();

      setTimeout(() => {
        const panel = document.getElementById("flashPanelCard");
        if (panel && panel.scrollIntoView) {
          panel.scrollIntoView({ behavior: "smooth", block: "nearest" });
        }
      }, 60);

      const startRes = await callBackend("flash", "POST", payload);
      if (startRes && startRes.started) {
        clearInterval(pollTimer);
        pollTimer = setInterval(async () => {
          if (state.pauseResumeBusy || !state.flashing || state.flashPaused || state.flashCancelled) {
            updateFlashPanelInPlace();
            return;
          }
          const reqSeq = state.flashOpSeq || 0;
          const snap = await callBackend("status", "GET");
          if (!snap) return;
          if (state.pauseResumeBusy || (state.flashOpSeq || 0) !== reqSeq || !state.flashing || state.flashCancelled) return;
          if (snap.close_warning_requested && window.__showCloseWhileFlashingWarning) {
            window.__showCloseWhileFlashingWarning();
          }
          if (state.flashPaused || (snap.status === "PAUSED" && (!state.lastResumeEpoch || Date.now() - state.lastResumeEpoch > 1500))) {
            state.flashPaused = true;
            updateFlashPanelInPlace();
            return;
          }

          spinIdx = (spinIdx + 1) % spinFrames.length;
          const elapsedSec = Math.max(0, Math.floor((Date.now() - flashStartEpoch) / 1000));
          const elapsedStr = `${String(Math.floor(elapsedSec / 60)).padStart(2, "0")}:${String(elapsedSec % 60).padStart(2, "0")}`;

          state.flashProgress = Math.max(state.flashProgress, Number(snap.percent) || 6);
          if (snap.active_progress && snap.active_progress.detail && snap.active_progress.detail.includes("Total ETA")) {
            const etaMatch = snap.active_progress.detail.match(/Total ETA\s+([0-9:]+)/);
            if (etaMatch) state.flashEtaText = `ETA: ~${etaMatch[1]} remaining`;
          } else if (state.flashProgress > 8 && state.flashProgress < 100) {
            const pctDone = Math.max(1, state.flashProgress - 5);
            const estTotalSec = Math.round((elapsedSec / pctDone) * (100 - state.flashProgress));
            const remMin = String(Math.floor(estTotalSec / 60)).padStart(2, "0");
            const remSec = String(estTotalSec % 60).padStart(2, "0");
            state.flashEtaText = `ETA: ~${remMin}:${remSec} remaining`;
          }

          if (Array.isArray(snap.events) && snap.events.length > 0) {
            const backendLines = [];
            let latestInPlaceFromEvents = null;
            snap.events.forEach((ev, idx) => {
              const evName = String(ev.event || "");
              const msg = String(ev.message || "");
              if (evName.endsWith("_PROGRESS") || (ev.data && ev.data.in_place)) {
                latestInPlaceFromEvents = ev;
              } else {
                const lineText = ev.formatted_line || `[00:${String(idx + 1).padStart(2, "0")}] ${msg}`;
                backendLines.push(lineText);
              }
            });
            state.flashLogs = [
              "[00:00] Saved config to Instant_Server_Data/instant-server-config.json",
              ...backendLines
            ];
            state.flashStatusText = snap.events[snap.events.length - 1].message || state.flashStatusText;

            if (snap.active_progress && (snap.active_progress.message || snap.active_progress.terminal_line)) {
              const stPct = Number(snap.active_progress.stage_percent ?? state.flashProgress);
              const stLbl = snap.active_progress.label || "Active Stage";
              const stMsg = snap.active_progress.message || state.flashStatusText;
              if (!state.lastWailsEventTs || (Date.now() - state.lastWailsEventTs) > 600) {
                state.activeProgressLine = buildAsciiTerminalBar(stPct, stLbl, stMsg).replace(/^⟳/, spinFrames[spinIdx]);
              }
              state.flashStatusText = stMsg;
            } else if (latestInPlaceFromEvents) {
              const d = latestInPlaceFromEvents.data || {};
              const stPct = Number(d.stage_percent ?? d.download_percent ?? d.percent ?? state.flashProgress);
              state.activeProgressLine = buildAsciiTerminalBar(stPct, d.label || "Progress", latestInPlaceFromEvents.message).replace(/^⟳/, spinFrames[spinIdx]);
              state.flashStatusText = latestInPlaceFromEvents.message;
            } else if (snap.status === "RUNNING") {
              state.activeProgressLine = buildAsciiTerminalBar(
                state.flashProgress,
                "Active Stage",
                `${state.flashStatusText} (Elapsed ${elapsedStr})`
              ).replace(/^⟳/, spinFrames[spinIdx]);
            }
          } else if (snap.status === "RUNNING") {
            state.activeProgressLine = buildAsciiTerminalBar(
              state.flashProgress,
              "Active Stage",
              `${state.flashStatusText} (Elapsed ${elapsedStr})`
            ).replace(/^⟳/, spinFrames[spinIdx]);
          }

          if (snap.status === "COMPLETED") {
            clearInterval(pollTimer);
            pollTimer = null;
            state.flashing = false;
            state.flashProgress = 100;
            state.activeProgressLine = "";
            state.flashEtaText = "";
            const usbInfo = (snap.result && snap.result.usb_provision) || {};
            const copiedCount = Array.isArray(usbInfo.copied_files) ? usbInfo.copied_files.length : 0;
            const mountName = usbInfo.target_mount || usbInfo.target_device || "USB drive";
            state.flashStatusText = "Bootable USB drive is ready! Plug it into your computer and power it on.";
            state.flashLogs.push(`[DONE] Verified bootable Ubuntu Server ISO + ${copiedCount} cidata & app file(s) on ${mountName}.`);
            showToast("Bootable Instant Server USB is ready");
          } else if (snap.status === "CANCELLED") {
            clearInterval(pollTimer);
            pollTimer = null;
            state.flashing = false;
            state.flashCancelled = true;
            state.activeProgressLine = "";
            state.flashStatusText = "Flashing cancelled by user — safe to unplug or retry anytime.";
          } else if (snap.status === "FAILED") {
            clearInterval(pollTimer);
            pollTimer = null;
            state.flashing = false;
            state.activeProgressLine = "";
            state.flashStatusText = `Failed: ${snap.error || "Check logs"}`;
            state.flashLogs.push(`[ERROR] ${snap.error || "Provisioning failed"}`);
          }
          updateFlashPanelInPlace();
        }, 350);
        return;
      }

      if (startRes && startRes.elevated_relaunch) {
        state.flashing = false;
        state.forceClosing = true;
        showToast("Administrator window launched — closing this unprivileged window...");
        setTimeout(() => { window.close(); }, 180);
        return;
      }
      if (window.location.protocol.startsWith("http") || window.pywebview) {
        state.flashing = false;
        state.activeProgressLine = "";
        const errReason = (startRes && startRes.reason) ? startRes.reason : "Could not start background USB flashing service.";
        state.flashStatusText = `Failed: ${errReason}`;
        state.flashLogs.push(`[ERROR] ${errReason}`);
        updateFlashPanelInPlace();
        return;
      }

      const selectedUsb = state.usbDevices.find(d => d.device_id === state.selectedUsbId);
      const usbLabel = selectedUsb ? `${selectedUsb.model} (${selectedUsb.device_node})` : "USB Drive";
      const schemeDesc = state.usb_partition_scheme === "mbr_bios"
        ? "MBR (Legacy BIOS + CSM active boot)"
        : "GPT + Hybrid UEFI/BIOS";

      const simPhases = [
        { type: "log", pct: 10, msg: `[00:03] Resolving official Ubuntu Server ${state.ubuntu_release}.1 LTS mirror & SHA256SUMS...` },
        { type: "bar", pct: 16, stagePct: 28.4, label: "Downloading Ubuntu ISO", detail: "749.8 / 2640.0 MB @ 24.5 MB/s" },
        { type: "bar", pct: 24, stagePct: 64.2, label: "Downloading Ubuntu ISO", detail: "1694.9 / 2640.0 MB @ 25.1 MB/s" },
        { type: "bar", pct: 34, stagePct: 100.0, label: "Downloading Ubuntu ISO", detail: "2640.0 / 2640.0 MB @ 25.0 MB/s" },
        { type: "log", pct: 40, msg: `[00:06] Official Ubuntu Server ${state.ubuntu_release}.1 LTS ISO SHA-256 checksum verified!` },
        { type: "log", pct: 46, msg: `[00:07] Formatting ${usbLabel} as ${schemeDesc} (label: CIDATA)...` },
        { type: "bar", pct: 58, stagePct: 42.0, label: "Writing Bootable ISO to USB", detail: "1092.0 / 2600.0 MB (casper/vmlinuz & initrd)" },
        { type: "bar", pct: 74, stagePct: 88.5, label: "Writing Bootable ISO to USB", detail: "2301.0 / 2600.0 MB (casper/ubuntu-server-minimal.squashfs)" },
        { type: "log", pct: 86, msg: `[00:10] Pre-unpacking ${state.selected_apps.length} self-hosted CasaOS app bundle(s) (${state.selected_apps.slice(0, 4).join(", ")}${state.selected_apps.length > 4 ? "..." : ""}) onto USB...` },
        { type: "done", pct: 100, msg: "[DONE] Bootable Ubuntu Server ISO + GRUB autoinstall + pre-unpacked apps written to USB." }
      ];
      let idx = 0;
      clearInterval(pollTimer);
      pollTimer = setInterval(() => {
        if (state.flashPaused) {
          updateFlashPanelInPlace();
          return;
        }
        if (!state.flashing || idx >= simPhases.length) {
          clearInterval(pollTimer);
          pollTimer = null;
          return;
        }
        const step = simPhases[idx++];
        state.flashProgress = step.pct;
        if (step.type === "log") {
          state.activeProgressLine = "";
          state.flashLogs.push(step.msg);
          state.flashStatusText = step.msg.replace(/^\[[0-9:]+\]\s*/, "");
        } else if (step.type === "bar") {
          state.activeProgressLine = buildAsciiTerminalBar(step.stagePct, step.label, step.detail);
          state.flashStatusText = `${step.label}: ${step.detail}`;
        } else if (step.type === "done") {
          state.activeProgressLine = "";
          state.flashLogs.push(step.msg);
          state.flashStatusText = "Bootable USB drive is ready! Plug it into your computer and power it on.";
          state.flashing = false;
          clearInterval(pollTimer);
          pollTimer = null;
          showToast("Bootable Instant Server USB is ready");
        }
        updateFlashPanelInPlace();
      }, 480);
    }

    function renderSwitch(action, isOn) {
      return `
        <div class="win-switch ${isOn ? "on" : ""}" data-action="${esc(action)}" role="switch" aria-checked="${isOn}">
          <span class="win-switch-label">${isOn ? "On" : "Off"}</span>
          <div class="win-switch-track"><div class="win-switch-thumb"></div></div>
        </div>
      `;
    }

    function renderSlider(field, min, max, step, val, unit = "%", widthPx = 160) {
      const numMin = Number(min) || 0;
      const numMax = Number(max) || 100;
      const numVal = Number(val ?? numMin);
      const ratio = Math.max(0, Math.min(1, (numVal - numMin) / Math.max(1, numMax - numMin)));
      const thumbOffsetPx = 8 + ratio * (widthPx - 16);
      return `
        <div class="win-slider-wrap" style="width:${widthPx}px;">
          <div class="win-slider-bubble" style="left:${thumbOffsetPx.toFixed(1)}px;">${esc(String(numVal))}${esc(unit)}</div>
          <input
            type="range"
            min="${min}"
            max="${max}"
            step="${step}"
            value="${numVal}"
            data-field="${esc(field)}"
            data-unit="${esc(unit)}"
            style="accent-color:var(--accent);width:${widthPx}px;"
          />
        </div>
      `;
    }

    function renderComboBox(field, options) {
      const currentVal = String(state[field] ?? "");
      const currentOpt = options.find(o => String(o.value) === currentVal) || options[0];
      const isOpen = state.openDropdownField === field;
      return `
        <div class="win-combobox ${isOpen ? "open" : ""}" data-combobox="${esc(field)}">
          <button type="button" class="win-combobox-trigger" data-action="toggle-dropdown" data-field="${esc(field)}">
            <span class="win-combobox-label">${esc(currentOpt ? currentOpt.label : "")}</span>
            <span class="win-combobox-chevron">⌄</span>
          </button>
          <div class="win-combobox-menu" role="listbox">
            ${options.map(opt => {
              const isSel = String(opt.value) === currentVal;
              return `
                <button
                  type="button"
                  class="win-combobox-option ${isSel ? "selected" : ""}"
                  data-action="select-dropdown-option"
                  data-field="${esc(field)}"
                  data-value="${esc(opt.value)}"
                  data-label="${esc(opt.label)}"
                  role="option"
                  aria-selected="${isSel}"
                >
                  <span>${esc(opt.label)}</span>
                </button>
              `;
            }).join("")}
          </div>
        </div>
      `;
    }

    function renderExpander(tabKey, title, innerHtml) {
      const isOpen = state.ui_mode === "pro" || !!state.advancedExpanded[tabKey];
      return `
        <div class="expander-wrap ${isOpen ? "open" : ""}">
          <button type="button" class="expander-header" data-action="toggle-expander" data-expander="${esc(tabKey)}">
            <span>${esc(title)}</span>
            <span class="expander-chevron">⌄</span>
          </button>
          <div class="expander-content-grid">
            <div class="expander-content-inner">
              <div class="expander-body">
                ${innerHtml}
              </div>
            </div>
          </div>
        </div>
      `;
    }

    function renderFlashProgressAndLogsCard() {
      const hasActivity = state.flashing || state.flashProgress > 0 || state.flashLogs.length > 0;
      const missingHtml = state.validationErrors.length > 0 ? `
        <div class="notice-banner">
          <strong>Complete the following before flashing:</strong>
          <ul style="padding-left: 18px; margin-top: 2px;">
            ${state.validationErrors.map(e => `<li>${esc(e)}</li>`).join("")}
          </ul>
        </div>
      ` : "";

      if (!hasActivity) {
        return missingHtml;
      }

      const badgeClass = state.flashProgress === 100
        ? "badge-success"
        : state.flashCancelled
        ? "badge-warning"
        : "badge-accent";

      return `
        ${missingHtml}
        <div class="flash-panel" id="flashPanelCard">
          <div style="display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;">
            <div style="display:flex;align-items:center;gap:10px;min-width:0;flex:1;">
              <span id="flashStatusTitle" style="font-weight:600;font-size:13.5px;">${esc(state.flashStatusText || "Preparing bootable USB...")}</span>
              <span id="flashBadgePct" class="badge ${badgeClass}">${state.flashProgress}%</span>
              <span id="flashEtaBadge" class="badge badge-accent" style="${state.flashEtaText ? "" : "display:none;"}">${esc(state.flashEtaText)}</span>
              <span id="flashConfigLockedBadge" class="badge badge-warning" style="${state.flashing ? "" : "display:none;"}">SEC Configuration Locked</span>
            </div>
            <div id="flashPanelHeaderActions" style="display:flex;align-items:center;gap:8px;">
              ${state.flashing ? `
                <button type="button" class="win-btn win-btn-sm ${state.flashPaused ? "win-btn-accent" : ""}" data-action="pause-resume-flash">
                  ${state.flashPaused ? `Resume (${state.flashProgress}%)` : "Pause"}
                </button>
                <button type="button" class="win-btn win-btn-sm win-btn-danger" data-action="cancel-flash">
                  Cancel
                </button>
              ` : ""}
            </div>
          </div>
          <div id="flashStageStepsRow" style="display:flex;align-items:center;gap:6px;flex-wrap:wrap;margin:2px 0;">
            ${renderFlashPipelineStagesHtml()}
          </div>
          <div class="progress-track">
            <div class="progress-fill" id="flashProgressFill" style="width: ${state.flashProgress}%;"></div>
          </div>
          <div class="flash-terminal-wrap">
            <div class="flash-log-box" id="flashLogBox" tabindex="0">
              <div id="flashLogLines">${state.flashLogs.map(l => `<div class="term-log-line">${esc(l)}</div>`).join("")}</div>
              <div id="flashTerminalLiveRow" class="term-live-progress" style="${state.activeProgressLine ? "" : "display:none;"}">${esc(state.activeProgressLine || "")}</div>
            </div>
            <button
              type="button"
              id="btnResumeAutoScroll"
              class="resume-scroll-pill"
              data-action="resume-log-autoscroll"
              style="${state.logAutoScroll ? "display:none;" : "display:inline-flex;"}"
            >
              ↓ Auto-scroll paused — Jump to bottom
            </button>
          </div>
          <details style="margin-top:6px;border-top:1px solid var(--border-subtle);padding-top:8px;">
            <summary style="cursor:pointer;font-size:12px;font-weight:600;color:var(--text-secondary);">
              Config and files overview
            </summary>
            <div class="code-viewer" style="margin-top:8px;max-height:170px;overflow:auto;">${esc(generateYamlPreview())}</div>
          </details>
        </div>
      `;
    }

    function buildSummaryLabels() {
      const modeNames = {
        dual_boot: `Dual-Boot (${state.dual_boot_Allocation_gb} GB)`,
        auto_erase: "Erase Main Drive",
        interactive_select: "Select on Screen",
        secondary_drive: "Secondary Drive Only",
        repair_rescue_usb: "Repair & Rescue USB"
      };
      const dashNames = {
        both: "Cockpit + CasaOS",
        cockpit: "Cockpit",
        casaos: "CasaOS",
        none: "SSH Only"
      };
      const usb = state.usbDevices.find(d => d.device_id === state.selectedUsbId && d.triple_lock_passed);
      return {
        modeLabel: modeNames[state.install_mode] || "Dual-Boot",
        dashLabel: `${dashNames[state.dashboard_choice] || "Cockpit + CasaOS"} (${state.selected_apps.length} apps)`,
        usbLabel: usb ? `${usb.model} (${usb.size_gb} GB)` : "No USB selected",
        wifiLabel: state.wifi_ssid.trim() ? state.wifi_ssid.trim() : "Ethernet / USB Tethering"
      };
    }

    function generateYamlPreview() {
      const sizingLine = state.storage_layout === "lvm" ? "\n      sizing-policy: all" : "";
      const storageSnippet = state.install_mode === "dual_boot"
        ? `  # Dual-Boot Mode (${state.dual_boot_Allocation_gb} GB): dynamic_dualboot_storage.sh shrinks existing OS safely\n  storage:\n    layout:\n      name: ${state.storage_layout}${sizingLine}\n      match:\n        size: largest`
        : state.install_mode === "interactive_select"
        ? `  interactive-sections:\n    - storage`
        : state.install_mode === "repair_rescue_usb"
        ? `  # Live Repair & Password Reset Mode: repairs bootloader/filesystem without formatting target drive`
        : state.install_mode === "offline_updater_usb"
        ? `  # Offline Updater USB Mode: updates dashboard, scripts, and incremental apps without formatting`
        : `  storage:\n    layout:\n      name: ${state.storage_layout}${sizingLine}\n      match:\n        size: largest`;

      return `#cloud-config
autoinstall:
  version: 1
  identity:
    hostname: ${state.hostname || "instant-server"}
    username: ${state.username || "admin"}
    password: "${state.password ? "$6$rounds=5000$hashed..." : "<set-password>"}"
  ssh:
    install-server: true
    allow-pw: true
${storageSnippet}
  # Pre-unpacked CasaOS apps on USB: ${state.selected_apps.join(", ") || "none"}
  late-commands:
    - curtin in-target -- bash /cdrom/cidata/laptop_hardening.sh`;
    }

    function buildNetworkSection() {
      const basicHtml = `
        <div class="card-stack">
          <div class="win-card">
            <div class="card-left">
              <div class="card-icon">LTS</div>
              <div class="card-text">
                <div class="card-title">Server name</div>
                <div class="card-desc" id="descServerHostname">Name used to reach your server on your home network (${esc(state.hostname || "instant-server")}.local)</div>
              </div>
            </div>
            <div class="card-right">
              <input class="win-input" data-field="hostname" value="${esc(state.hostname)}" placeholder="instant-server" />
            </div>
          </div>

          <div class="win-card">
            <div class="card-left">
              <div class="card-icon">USR</div>
              <div class="card-text">
                <div class="card-title">Admin username &amp; password</div>
                <div class="card-desc">Login credentials for the web dashboard and SSH</div>
              </div>
            </div>
            <div class="card-right" style="flex-wrap:wrap;">
              <input class="win-input" style="min-width:140px;" data-field="username" value="${esc(state.username)}" placeholder="admin" />
              <input class="win-input" style="min-width:170px;" type="password" data-field="password" value="${esc(state.password)}" placeholder="Enter password" />
            </div>
          </div>

          <div class="win-card" style="flex-direction:column;align-items:stretch;gap:10px;">
            <div style="display:flex;align-items:center;justify-content:space-between;gap:14px;flex-wrap:wrap;">
              <div class="card-left">
                <div class="card-icon">NET</div>
                <div class="card-text">
                  <div class="card-title">
                    <span>Wi-Fi network &amp; backup failover</span>
                    <span class="badge badge-warning">Ethernet Recommended for Media &amp; Backups</span>
                  </div>
                  <div class="card-desc">Leave blank if using Ethernet only, or enter Wi-Fi as an automatic failover backup</div>
                </div>
              </div>
              <div class="card-right" style="flex-wrap:wrap;">
                <input class="win-input" style="min-width:155px;" data-field="wifi_ssid" value="${esc(state.wifi_ssid)}" placeholder="Wi-Fi name (SSID)" />
                <input class="win-input" style="min-width:155px;" type="password" data-field="wifi_password" value="${esc(state.wifi_password)}" placeholder="Wi-Fi password" />
              </div>
            </div>
            <div style="font-size:12px;color:var(--text-secondary);padding-top:8px;border-top:1px solid var(--divider);line-height:1.45;">
              <strong>Tip:</strong> Plug in an Ethernet cable when possible for faster file transfers and streaming; Wi-Fi remains active as automatic backup failover.
            </div>
          </div>

          <div class="win-card" style="flex-direction:column;align-items:stretch;gap:10px;">
            <div style="display:flex;align-items:center;justify-content:space-between;gap:14px;flex-wrap:wrap;">
              <div class="card-left">
                <div class="card-icon">NET</div>
                <div class="card-text">
                  <div class="card-title">
                    <span>Remote access outside home (Tailscale VPN)</span>
                    <span class="badge badge-accent">Zero Port-Forwarding</span>
                  </div>
                  <div class="card-desc">Reach your server, dashboard, and self-hosted apps securely from anywhere in the world</div>
                </div>
              </div>
              <div class="card-right">
                ${renderSwitch("toggle-tailscale", state.enable_tailscale)}
              </div>
            </div>
            <div id="tailscaleNetConfigRow" style="display:${state.enable_tailscale ? "flex" : "none"};align-items:center;justify-content:space-between;gap:10px;flex-wrap:wrap;padding-top:8px;border-top:1px solid var(--divider);">
              <input
                id="tailscaleAuthInputNet"
                class="win-input"
                style="flex:1;min-width:220px;"
                data-field="tailscale_authkey"
                value="${esc(state.tailscale_authkey)}"
                placeholder="Pre-auth key: tskey-auth-... (optional — or sign in later in dashboard)"
              />
              ${renderComboBox("tailscale_mode", [
                { value: "subnet_and_ssh", label: "Home LAN Subnet Router + Tailscale SSH (Recommended)" },
                { value: "apps_and_ssh", label: "Server Apps & Tailscale SSH Only" },
                { value: "exit_node_full", label: "Subnet Router + Advertise as Exit Node" }
              ])}
            </div>
          </div>
        </div>
      `;

      const clusterStorageHtml = state.cluster_mode !== "standalone" ? `
        <div class="win-card">
          <div class="card-left">
            <div class="card-icon">NAS</div>
            <div class="card-text">
              <div class="card-title">Existing files on the second computer</div>
              <div class="card-desc">Choose whether to keep all existing files on both computers or start with empty space on the second one</div>
            </div>
          </div>
          <div class="card-right">
            ${renderComboBox("cluster_storage_policy", [
              { value: "mergerfs_union_safe", label: "Keep all files on both computers (Combined storage)" },
              { value: "replicated_ha_mirror", label: "Keep all files & mirror backups across both" },
              { value: "wipe_secondary_expand", label: "Erase second computer for empty space" }
            ])}
          </div>
        </div>
        ${state.cluster_storage_policy === "wipe_secondary_expand" ? `
          <div class="notice-banner" id="bannerWipeSecondary">
            <div style="font-weight:600;">Warning: All existing files on the second computer will be erased when it joins</div>
            <label style="display:flex;align-items:center;gap:8px;cursor:pointer;margin-top:4px;">
              <input type="checkbox" data-action="toggle-confirm-wipe-secondary" ${state.confirm_wipe_secondary ? "checked" : ""} style="accent-color:var(--danger);width:16px;height:16px;" />
              <span>I understand — erase the second computer's files when combining servers</span>
            </label>
          </div>
        ` : ""}
      ` : "";

      const advHtml = `
        <div class="win-card">
          <div class="card-left">
            <div class="card-icon">URL</div>
            <div class="card-text">
              <div class="card-title">Combine multiple computers into one server</div>
              <div class="card-desc">Use this same USB stick on multiple laptops/PCs to link them automatically, or connect another server later in the web dashboard</div>
            </div>
          </div>
          <div class="card-right">
            ${renderComboBox("cluster_mode", [
              { value: "standalone", label: "Single computer (Can still link another server later)" },
              { value: "usb_multi_node_auto", label: "Use this same USB on multiple computers (Auto-connect)" },
              { value: "cluster_worker", label: "Add this computer to an existing Instant Server" }
            ])}
          </div>
        </div>
        ${clusterStorageHtml}

        <div class="win-card">
          <div class="card-left">
            <div class="card-icon">MOB</div>
            <div class="card-text">
              <div class="card-title">Phone USB tethering</div>
              <div class="card-desc">Automatically use internet from a plugged-in Android or iPhone if Wi-Fi is unavailable</div>
            </div>
          </div>
          <div class="card-right">
            ${renderSwitch("toggle-tethering", state.enable_usb_tethering)}
          </div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-icon">UPD</div>
            <div class="card-text">
              <div class="card-title">Seamless Ethernet-to-Wi-Fi failover</div>
              <div class="card-desc">Keeps Wi-Fi awake while Ethernet is plugged in so unplugging the cable never drops your connection</div>
            </div>
          </div>
          <div class="card-right">
            ${renderSwitch("toggle-wifi-powersave", state.disable_wifi_power_save)}
          </div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-icon">CLU</div>
            <div class="card-text">
              <div class="card-title">Static IP address</div>
              <div class="card-desc">Optional fixed IP (leave empty for automatic DHCP)</div>
            </div>
          </div>
          <div class="card-right" style="flex-wrap:wrap;">
            <input class="win-input" style="min-width:145px;" data-field="static_ip_cidr" value="${esc(state.static_ip_cidr)}" placeholder="IP / CIDR" />
            <input class="win-input" style="min-width:135px;" data-field="static_gateway" value="${esc(state.static_gateway)}" placeholder="Gateway" />
          </div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-icon">SSH</div>
            <div class="card-text">
              <div class="card-title">SSH public key</div>
              <div class="card-desc">Optional key for passwordless SSH login</div>
            </div>
          </div>
          <div class="card-right">
            <input class="win-input" data-field="ssh_authorized_key" value="${esc(state.ssh_authorized_key)}" placeholder="ssh-ed25519 AAAA..." />
          </div>
        </div>

        ${state.ui_mode === "pro" ? `
          <div class="win-card">
            <div class="card-left">
              <div class="card-icon"></div>
              <div class="card-text">
                <div class="card-title">TCP BBR fast network streaming</div>
                <div class="card-desc">Improves local network and VPN speed for Jellyfin video streaming and Nextcloud file transfers</div>
              </div>
            </div>
            <div class="card-right">${renderSwitch("toggle-tcp-bbr", state.enable_tcp_bbr)}</div>
          </div>

          <div class="win-card">
            <div class="card-left">
              <div class="card-icon">FW</div>
              <div class="card-text">
                <div class="card-title">Firewall &amp; SSH login protection</div>
                <div class="card-desc">Blocks repeated failed SSH login attempts and protects open ports</div>
              </div>
            </div>
            <div class="card-right" style="gap:10px;">
              <input class="win-input" style="width:90px;min-width:90px;" data-field="ssh_port" value="${esc(state.ssh_port)}" placeholder="Port 22" title="Custom SSH Port" />
              ${renderSwitch("toggle-fail2ban", state.enable_fail2ban)}
            </div>
          </div>
        ` : ""}
      `;

      return `
        <div class="tab-flow-stack">
          ${basicHtml}
          ${renderExpander("network", "Advanced options", advHtml)}
        </div>
      `;
    }

    function doesAppMatchCategory(app, cat) {
      if (!cat || cat === "All") return true;
      const appCats = Array.isArray(app.categories) ? app.categories : [app.category];
      if (cat === "Recommended") return !!app.recommended;
      if (cat === "Old Laptop Friendly") return app.hardware_tier === "old_friendly";
      if (cat === "Higher-End / Gaming & AI") {
        return app.hardware_tier === "high_end" || appCats.includes("Gaming Servers") || appCats.includes("AI & Machine Learning");
      }
      return app.category === cat || appCats.includes(cat);
    }

    function getFilteredAppsList() {
      const q = state.appSearchQuery.trim().toLowerCase();
      const cat = state.appCategoryFilter;
      return APP_STORE_CATALOG.filter(app => {
        if (!doesAppMatchCategory(app, cat)) return false;
        if (q) {
          const appCatsStr = Array.isArray(app.categories) ? app.categories.join(" ") : app.category;
          const hay = (app.title + " " + app.desc + " " + app.tag + " " + appCatsStr + " " + (app.image || "") + " " + (app.hardware_note || "")).toLowerCase();
          if (!hay.includes(q)) return false;
        }
        return true;
      });
    }

    function renderFilteredAppCardsHtml() {
      const filtered = getFilteredAppsList();

      if (filtered.length === 0) {
        return `<div class="win-card"><div class="card-title">No apps match your search filter.</div></div>`;
      }

      const initialLimit = 8;
      const canCollapse = !state.appSearchQuery.trim() && state.appCategoryFilter === "All" && filtered.length > initialLimit;
      const visibleApps = (canCollapse && !state.appsExpanded) ? filtered.slice(0, initialLimit) : filtered;

      const cardsHtml = visibleApps.map(app => {
        const isChecked = state.selected_apps.includes(app.id);
        const isHighEnd = app.hardware_tier === "high_end";
        return `
          <div class="app-card ${isChecked ? "checked" : ""}" data-action="toggle-app-card" data-app="${esc(app.id)}">
            <div style="display:flex;align-items:flex-start;gap:12px;min-width:0;flex:1;">
              <input type="checkbox" class="app-checkbox" ${isChecked ? "checked" : ""} />
              <div class="card-text">
                <div class="card-title">
                  <span>${esc(app.title)}</span>
                  ${app.recommended ? `<span class="badge badge-accent">Recommended</span>` : ""}
                  <span class="badge badge-success">${esc(app.tag)} • ${esc(app.ram)}</span>
                  <span class="badge" style="background:var(--card-bg-secondary);border:1px solid var(--border-card);color:var(--text-secondary);">${esc(app.category)}</span>
                  ${isHighEnd ? `<span class="badge badge-warning">${esc(app.hardware_note || "8GB+ RAM")}</span>` : ""}
                </div>
                <div class="card-desc">${esc(app.desc)}</div>
              </div>
            </div>
          </div>
        `;
      }).join("");

      const expandBtnHtml = canCollapse ? `
        <div style="grid-column: 1 / -1; display:flex; justify-content:center; padding-top:4px;">
          <button type="button" class="win-btn" data-action="toggle-expand-apps">
            ${state.appsExpanded ? "Show fewer apps ⌃" : `Show more apps (${filtered.length - initialLimit} more) ⌄`}
          </button>
        </div>
      ` : "";

      return cardsHtml + expandBtnHtml;
    }


    function getOrInitAppOverride(appId) {
      const found = APP_STORE_CATALOG.find(a => a.id === appId) || APP_STORE_CATALOG[0];
      if (!state.app_overrides[appId]) {
        const tpl = DEFAULT_APP_CONFIG_TEMPLATES[appId] || {
          ".env": `TZ=UTC\nPUID=1000\nPGID=1000\nAPP_PORT=${found ? found.port : 8080}\n`,
          "custom-config.conf": `# Persistent configuration file in /DATA/AppData/${appId}\n`
        };
        state.app_overrides[appId] = {
          host_port: found ? found.port : 8080,
          memory_limit: "",
          env_vars: {},
          config_files: Object.assign({}, tpl)
        };
      }
      return state.app_overrides[appId];
    }

    function buildUniversalAppFileStudioHtml() {
      const activeAppId = state.studio_app_id || (state.selected_apps[0] || "minecraft");
      const appObj = APP_STORE_CATALOG.find(a => a.id === activeAppId) || APP_STORE_CATALOG[0];
      const ov = getOrInitAppOverride(appObj.id);
      const fileKeys = Object.keys(ov.config_files || {});
      if (!state.studio_file_name || !ov.config_files[state.studio_file_name]) {
        state.studio_file_name = fileKeys[0] || ".env";
      }
      const currentFileContent = ov.config_files[state.studio_file_name] ?? "";

      const prioritizedApps = [
        ...APP_STORE_CATALOG.filter(a => state.selected_apps.includes(a.id)),
        ...APP_STORE_CATALOG.filter(a => !state.selected_apps.includes(a.id))
      ];

      return `
        <div class="win-card" style="flex-direction:column;align-items:stretch;gap:12px;">
          <div style="display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;">
            <div class="card-left">
              <div class="card-icon">EXT4</div>
              <div class="card-text">
                <div class="card-title">
                  <span>Universal App Config &amp; <code>/DATA/AppData</code> File Studio</span>
                  <span class="badge badge-accent">All ${APP_STORE_CATALOG.length} Apps Supported</span>
                </div>
                <div class="card-desc">Pre-configure ports, RAM limits, and persistent config files (<code>server.properties</code>, <code>config.php</code>, <code>pihole-FTL.conf</code>, <code>.env</code>) for any app</div>
              </div>
            </div>
            <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap;">
              <select class="win-input" style="min-width:210px;" data-action="select-studio-app">
                ${prioritizedApps.map(a => `
                  <option value="${esc(a.id)}" ${a.id === appObj.id ? "selected" : ""}>
                    ${state.selected_apps.includes(a.id) ? " " : ""}${esc(a.title)} (Port ${a.port || 8080})
                  </option>
                `).join("")}
              </select>
            </div>
          </div>

          <div style="display:flex;align-items:center;justify-content:space-between;gap:10px;flex-wrap:wrap;padding-top:8px;border-top:1px solid var(--divider);">
            <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap;">
              <span style="font-size:12px;font-weight:600;color:var(--text-secondary);">Host Port:</span>
              <input type="number" class="win-input" style="width:95px;min-width:95px;" data-action="studio-port-input" value="${esc(ov.host_port)}" />
              <span style="font-size:12px;font-weight:600;color:var(--text-secondary);">RAM Cap:</span>
              <input type="text" class="win-input" style="width:105px;min-width:105px;" data-action="studio-mem-input" value="${esc(ov.memory_limit)}" placeholder="Unlimited (e.g. 2g)" />
            </div>
            <div style="display:flex;align-items:center;gap:6px;flex-wrap:wrap;">
              ${fileKeys.map(fn => `
                <button type="button" class="filter-pill ${state.studio_file_name === fn ? "active" : ""}" data-action="select-studio-file" data-file="${esc(fn)}">
                  CFG ${esc(fn)}
                </button>
              `).join("")}
              <input type="text" id="inputNewStudioFile" class="win-input" style="width:135px;min-width:135px;height:28px;font-size:12px;" placeholder="new-file.conf" value="${esc(state.studio_new_file_name || "")}" data-field="studio_new_file_name" />
              <button type="button" class="win-btn win-btn-sm" data-action="add-studio-file">＋ New File</button>
            </div>
          </div>

          <div>
            <div style="display:flex;align-items:center;justify-content:space-between;font-size:11.5px;color:var(--text-secondary);margin-bottom:4px;">
              <span>Editing <code>/DATA/AppData/${esc(appObj.id)}/${esc(state.studio_file_name)}</code> (pre-staged onto USB &amp; mounted into container)</span>
              <span>Image: <code>${esc(appObj.image || appObj.id)}</code></span>
            </div>
            <textarea
              class="win-input"
              style="width:100%;min-height:115px;padding:10px 12px;font-family:var(--font-mono);font-size:12px;line-height:1.5;resize:vertical;"
              data-action="studio-file-textarea"
              spellcheck="false"
            >${esc(currentFileContent)}</textarea>
          </div>
        </div>
      `;
    }

    function buildAppsSection() {
      const dashboards = [
        { id: "both", icon: "", title: "Cockpit + CasaOS", desc: "System web console plus the CasaOS 1-click app store (install even more apps anytime later)", rec: true },
        { id: "cockpit", icon: "PRO", title: "Cockpit", desc: "Lightweight system manager for storage, network, and terminal", rec: false },
        { id: "casaos", icon: "DKR", title: "CasaOS", desc: "Self-hosted home cloud dashboard and 1-click Docker app store", rec: false },
        { id: "none", icon: "CLI", title: "SSH Only", desc: "Minimal terminal server with no web dashboard", rec: false }
      ];

      const estRamMb = state.selected_apps.reduce((acc, id) => {
        const found = APP_STORE_CATALOG.find(a => a.id === id);
        if (!found) return acc;
        const m = String(found.ram || "128 MB").match(/([0-9.]+)\s*(GB|MB)/i);
        if (!m) return acc + 128;
        return acc + (m[2].toUpperCase() === "GB" ? Number(m[1]) * 1024 : Number(m[1]));
      }, 420);
      const estRamGb = (estRamMb / 1024).toFixed(1);
      const estWatts = state.device_form_factor === "mini_pc_n100" ? "5.2W" : (state.cpu_governor === "powersave" ? "9.4W" : "11.8W");

      const basicHtml = `
        <div class="section-label">1. One-click server role presets (Or customize individual apps below)</div>
        <div class="presets-grid">
          ${SERVER_ROLE_PRESETS.map(p => `
            <div class="preset-card ${state.preset_role === p.id ? "selected" : ""}" data-action="apply-role-preset" data-preset="${esc(p.id)}">
              <div style="display:flex;align-items:center;justify-content:space-between;gap:6px;">
                <span style="font-weight:600;font-size:13px;">${esc(p.title)}</span>
                <span class="badge badge-accent">${esc(p.badge)}</span>
              </div>
              <div style="font-size:11.5px;color:var(--text-secondary);line-height:1.4;">${esc(p.desc)}</div>
            </div>
          `).join("")}
        </div>

        <div class="win-card" style="border-color:var(--accent);">
          <div class="card-left">
            <div class="card-icon">MON</div>
            <div class="card-text">
              <div class="card-title">Estimated Memory &amp; Server Homepage</div>
              <div class="card-desc">Selected apps (${state.selected_apps.length}) use ~<strong>${estRamGb} GB RAM</strong> • Idle power ~<strong>${estWatts}</strong> • Includes Server Homepage at <code>http://${esc(state.hostname || "instant-server")}.local:${state.dashboard_port}</code></div>
            </div>
          </div>
          <div class="card-right">
            <span class="badge badge-success">${Number(estRamGb) <= 3.5 ? "Fits 4GB+ Computers" : "8GB+ RAM Recommended"}</span>
          </div>
        </div>

        <div class="section-label">2. Primary web dashboard</div>
        <div class="card-stack">
          ${dashboards.map(d => {
            const active = state.dashboard_choice === d.id;
            return `
              <div class="win-card clickable ${active ? "selected-accent" : ""}" data-action="select-dashboard" data-value="${d.id}">
                <div class="card-left">
                  <div class="card-icon">${d.icon}</div>
                  <div class="card-text">
                    <div class="card-title">
                      ${esc(d.title)}
                      ${d.rec ? `<span class="badge badge-accent">Recommended</span>` : ""}
                    </div>
                    <div class="card-desc">${esc(d.desc)}</div>
                  </div>
                </div>
                <div class="card-right">
                  <input type="radio" name="dash_choice" ${active ? "checked" : ""} style="accent-color:var(--accent);width:17px;height:17px;pointer-events:none;" />
                </div>
              </div>
            `;
          }).join("")}
        </div>

        <div style="display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:8px;margin-top:4px;">
          <div>
            <div class="section-label" style="margin:0;">3. Pre-select self-hosted apps to bundle onto USB (<span id="lblSelectedAppsCount">${state.selected_apps.length}</span> selected)</div>
            <div style="font-size:12px;color:var(--text-secondary);">Selected apps are staged onto your USB drive and automatically set up in CasaOS &amp; Docker.</div>
          </div>
          <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap;">
            <label class="select-all-checkbox-pill" id="chkSelectAllAppsWrap" data-action="toggle-select-all-apps" title="Toggle selecting or unselecting all apps">
              <input type="checkbox" id="chkSelectAllApps" ${state.selected_apps.length >= APP_STORE_CATALOG.length ? "checked" : ""} />
              <span id="lblSelectAllAppsText">${state.selected_apps.length >= APP_STORE_CATALOG.length ? "Unselect all apps" : `Select all (${state.selected_apps.length}/${APP_STORE_CATALOG.length})`}</span>
            </label>
            <button type="button" class="win-btn win-btn-sm" data-action="apps-select-recommended">Recommended</button>
            <button type="button" class="win-btn win-btn-sm" data-action="apps-select-old-laptop">Old Laptop Pack</button>
            <button type="button" class="win-btn win-btn-sm" data-action="apps-select-high-end">Higher-End + AI &amp; Gaming</button>
          </div>
        </div>

        <div class="win-card" style="padding:10px 14px;">
          <div class="card-left">
            <div class="card-icon">NET</div>
            <div class="card-text">
              <div class="card-title">Add custom app or any app from the web</div>
              <div class="card-desc">Paste a Docker image, <code>docker run</code> command, or GitHub / Docker Hub URL</div>
            </div>
          </div>
          <div class="card-right" style="flex:1;max-width:460px;display:flex;gap:8px;">
            <input
              type="text"
              class="win-input"
              style="flex:1;min-width:200px;"
              id="customWebAppInput"
              data-field="customWebAppInput"
              value="${esc(state.customWebAppInput || "")}"
              placeholder="e.g. linuxserver/plex:latest, docker run -p 8080:80 ..., or GitHub URL"
            />
            <button type="button" class="win-btn win-btn-accent" data-action="add-custom-web-app">＋ Add App</button>
          </div>
        </div>

        <div class="appstore-toolbar">
          <div class="appstore-top-row">
            <input
              type="text"
              class="win-input"
              style="flex:1;min-width:220px;"
              id="appStoreSearchInput"
              data-field="appSearchQuery"
              value="${esc(state.appSearchQuery)}"
              placeholder="Search ${APP_STORE_CATALOG.length}+ self-hosted apps (Jellyfin, Nextcloud, Immich, Pi-hole, Ollama, Minecraft, Grafana, Frigate)..."
            />
          </div>
          <div class="appstore-filter-pills" id="appCategoryPills">
            ${APP_CATEGORIES.map(cat => {
              const cnt = APP_STORE_CATALOG.filter(a => doesAppMatchCategory(a, cat)).length;
              return `
                <button type="button" class="filter-pill ${state.appCategoryFilter === cat ? "active" : ""}" data-action="filter-app-category" data-category="${esc(cat)}">
                  ${esc(cat)} <span style="opacity:0.75;font-size:11px;margin-left:3px;">(${cnt})</span>
                </button>
              `;
            }).join("")}
          </div>
        </div>

        
        <div class="win-card" id="minecraftCustomizationStudioCard" style="flex-direction:column;align-items:stretch;gap:12px;border-color:var(--accent);display:${(state.selected_apps.includes("minecraft") || state.selected_apps.includes("minecraft_bedrock")) ? "flex" : "none"};">
          <div style="display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;">
            <div class="card-left">
              <div class="card-icon">MC</div>
              <div class="card-text">
                <div class="card-title">
                  <span>Minecraft Server Settings</span>
                  <span class="badge badge-accent">Java &amp; Bedrock</span>
                </div>
                <div class="card-desc">Quick setup for Minecraft <strong>${esc(state.mc_version || "26.3")}</strong>. Manage worlds, files, and console later in the Web Dashboard.</div>
              </div>
            </div>
          </div>

          <div style="display:grid;grid-template-columns:repeat(auto-fit, minmax(210px, 1fr));gap:12px;padding-top:8px;border-top:1px solid var(--divider);">
            <div>
              <div style="font-size:11.5px;font-weight:600;color:var(--text-secondary);margin-bottom:4px;">Server Type</div>
              ${renderComboBox("mc_server_type", [
                { value: "PAPER", label: "Paper (Fastest + Plugins)" },
                { value: "GEYSER_FLOODGATE", label: "Paper + Bedrock Crossplay" },
                { value: "PURPUR", label: "Purpur" },
                { value: "FABRIC", label: "Fabric (Mods)" },
                { value: "FORGE", label: "Forge (Modpacks)" },
                { value: "NEOFORGE", label: "NeoForge" },
                { value: "VANILLA", label: "Vanilla" }
              ])}
            </div>

            <div>
              <div style="font-size:11.5px;font-weight:600;color:var(--text-secondary);margin-bottom:4px;">Version</div>
              ${renderComboBox("mc_version", [
                { value: "26.3", label: "26.3 (Latest)" },
                { value: "26.2", label: "26.2" },
                { value: "26.1", label: "26.1" },
                { value: "1.21.11", label: "1.21.11" },
                { value: "1.21.4", label: "1.21.4" },
                { value: "LATEST", label: "Auto-Latest" }
              ])}
            </div>

            <div>
              <div style="font-size:11.5px;font-weight:600;color:var(--text-secondary);margin-bottom:4px;">Memory (RAM)</div>
              ${renderComboBox("mc_memory_heap", [
                { value: "2G", label: "2 GB RAM" },
                { value: "4G", label: "4 GB RAM (Recommended)" },
                { value: "6G", label: "6 GB RAM" },
                { value: "8G", label: "8 GB RAM" },
                { value: "12G", label: "12 GB RAM" }
              ])}
            </div>

            <div>
              <div style="font-size:11.5px;font-weight:600;color:var(--text-secondary);margin-bottom:4px;">Game Mode</div>
              ${renderComboBox("mc_gamemode", [
                { value: "survival", label: "Survival" },
                { value: "creative", label: "Creative" },
                { value: "adventure", label: "Adventure" }
              ])}
            </div>

            <div>
              <div style="font-size:11.5px;font-weight:600;color:var(--text-secondary);margin-bottom:4px;">Difficulty</div>
              ${renderComboBox("mc_difficulty", [
                { value: "normal", label: "Normal" },
                { value: "hard", label: "Hard" },
                { value: "easy", label: "Easy" },
                { value: "peaceful", label: "Peaceful" }
              ])}
            </div>

            <div>
              <div style="font-size:11.5px;font-weight:600;color:var(--text-secondary);margin-bottom:4px;">Max Players &amp; Admins</div>
              <div style="display:flex;gap:6px;min-width:0;">
                <input class="win-input" type="number" style="width:74px;min-width:0;flex-shrink:0;" data-field="mc_max_players" value="${esc(state.mc_max_players)}" placeholder="20" title="Max Players" />
                <input class="win-input" style="flex:1;min-width:0;" data-field="mc_ops_list" value="${esc(state.mc_ops_list)}" placeholder="Admin usernames" />
              </div>
            </div>

            <div>
              <div style="font-size:11.5px;font-weight:600;color:var(--text-secondary);margin-bottom:4px;">Plugins / Mods (Optional)</div>
              <input class="win-input" style="width:100%;min-width:0;" data-field="mc_modrinth_projects" value="${esc(state.mc_modrinth_projects)}" placeholder="e.g. viaversion, worldedit" />
            </div>

            <div>
              <div style="font-size:11.5px;font-weight:600;color:var(--text-secondary);margin-bottom:4px;">World Seed &amp; Server Name</div>
              <div style="display:flex;gap:6px;min-width:0;">
                <input class="win-input" style="width:96px;min-width:0;flex-shrink:0;" data-field="mc_level_seed" value="${esc(state.mc_level_seed)}" placeholder="Seed" />
                <input class="win-input" style="flex:1;min-width:0;" data-field="mc_motd" value="${esc(state.mc_motd)}" placeholder="Server name" />
              </div>
            </div>
          </div>

          <div style="display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;padding-top:8px;border-top:1px solid var(--divider);font-size:12px;">
            <label style="display:flex;align-items:center;gap:8px;cursor:pointer;">
              <input type="checkbox" data-action="toggle-mc-online-mode" ${state.mc_online_mode ? "checked" : ""} style="accent-color:var(--accent);width:16px;height:16px;" />
              <span>Require Mojang sign-in (uncheck for offline/LAN players)</span>
            </label>
            <label style="display:flex;align-items:center;gap:8px;cursor:pointer;">
              <input type="checkbox" data-action="toggle-mc-aikar" ${state.mc_use_aikar_flags ? "checked" : ""} style="accent-color:var(--accent);width:16px;height:16px;" />
              <span>Performance flags &amp; automatic backups</span>
            </label>
          </div>
        </div>

      `;

      const advHtml = `
        <div class="win-card">
          <div class="card-left">
            <div class="card-icon">PKG</div>
            <div class="card-text">
              <div class="card-title">Pre-download container images on this PC (Offline bundle)</div>
              <div class="card-desc">Downloads the full Docker image layers onto the USB stick from this PC so apps install even without internet</div>
            </div>
          </div>
          <div class="card-right">${renderSwitch("toggle-offline-images", state.prebundle_offline_images)}</div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-icon"></div>
            <div class="card-text">
              <div class="card-title">Pi-hole DNS</div>
              <div class="card-desc">Prepare DNS settings so Pi-hole or AdGuard Home can run without port conflicts</div>
            </div>
          </div>
          <div class="card-right">${renderSwitch("toggle-pihole", state.enable_pihole_dns_prep)}</div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-icon">DIR</div>
            <div class="card-text">
              <div class="card-title">Samba File Share</div>
              <div class="card-desc">Share folders across Windows, macOS, and Linux computers on your home network</div>
            </div>
          </div>
          <div class="card-right">${renderSwitch("toggle-samba", state.enable_samba_nas)}</div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-icon">VID</div>
            <div class="card-text">
              <div class="card-title">Hardware Video Transcoding</div>
              <div class="card-desc">Enable Intel, AMD, or NVIDIA graphics acceleration for Jellyfin and Plex</div>
            </div>
          </div>
          <div class="card-right">${renderSwitch("toggle-vaapi", state.enable_intel_vaapi)}</div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-icon">SEC</div>
            <div class="card-text">
              <div class="card-title">Tailscale VPN</div>
              <div class="card-desc">Access your server remotely without opening router ports</div>
            </div>
          </div>
          <div class="card-right" style="flex-wrap:wrap;">
            <input
              id="tailscaleAuthInput"
              class="win-input"
              style="min-width:160px; ${state.enable_tailscale ? "" : "display:none;"}"
              data-field="tailscale_authkey"
              value="${esc(state.tailscale_authkey)}"
              placeholder="Auth key (optional)"
            />
            ${renderSwitch("toggle-tailscale", state.enable_tailscale)}
          </div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-icon">ZIM</div>
            <div class="card-text">
              <div class="card-title">Offline Wikipedia &amp; survival pack for Kiwix</div>
              <div class="card-desc">Choose which offline encyclopedia (.zim) to pre-configure in your Server Homepage &amp; Kiwix library</div>
            </div>
          </div>
          <div class="card-right">
            ${renderComboBox("preselect_kiwix_zim", KIWIX_ZIM_PACKS.map(z => ({ value: z.id, label: z.label })))}
          </div>
        </div>

        <div class="win-card" style="flex-direction:column;align-items:stretch;gap:10px;border:1px solid var(--accent);">
          <div style="display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;">
            <div class="card-left">
              <div class="card-icon">FIX</div>
              <div class="card-text">
                <div class="card-title">Custom Server Workflow &amp; Automation Builder <span class="badge badge-accent">Full Control</span></div>
                <div class="card-desc">Build any custom server workflow: install extra Ubuntu packages (<code>apt</code>), create custom Samba NAS shares, add USB hardware trigger rules (<code>/etc/udev/rules.d/</code>), add background <code>systemd</code> services/timers, or stage any <code>/etc</code> / <code>/DATA</code> config file &amp; post-install script (${Object.keys(state.custom_config_files || {}).length} file(s) staged).</div>
              </div>
            </div>
            <div class="card-right" style="gap:6px;flex-wrap:wrap;">
              <button type="button" class="win-btn win-btn-sm" data-action="load-workflow-template" data-tpl="samba_share">DIR + Custom NAS Share</button>
              <button type="button" class="win-btn win-btn-sm" data-action="load-workflow-template" data-tpl="usb_udev">UPS + USB Device Trigger</button>
              <button type="button" class="win-btn win-btn-sm" data-action="load-workflow-template" data-tpl="systemd_timer">TMR + Scheduled Service</button>
            </div>
          </div>
          <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap;padding-top:6px;border-top:1px solid var(--divider);">
            <span style="font-size:12px;font-weight:600;color:var(--text-secondary);">Extra Ubuntu Packages (apt):</span>
            <input class="win-input" style="flex:1;min-width:180px;" data-field="custom_apt_packages_raw" value="${esc(state.custom_apt_packages_raw || "")}" placeholder="e.g. ffmpeg rsync python3-pip rclone usbmuxd" />
            <span style="font-size:12px;font-weight:600;color:var(--text-secondary);">Target File Path:</span>
            <input class="win-input" style="min-width:210px;" data-field="custom_config_path" value="${esc(state.custom_config_path)}" placeholder="/etc/samba/smb-custom.conf or /etc/udev/rules.d/99-usb.rules" />
            <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="save-custom-sys-config">＋ Stage Custom File</button>
          </div>
          <textarea class="win-input" rows="4" style="width:100%;font-family:var(--font-mono);font-size:12px;" data-field="custom_config_body" placeholder="Paste custom config file, systemd unit, udev rule, or automation script here...">${esc(state.custom_config_body)}</textarea>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-icon">UPD</div>
            <div class="card-text">
              <div class="card-title">Automatic OS security &amp; web app updates</div>
              <div class="card-desc">Automatically installs Ubuntu security patches and updates Docker apps at ${esc(state.auto_update_window)}</div>
            </div>
          </div>
          <div class="card-right">${renderSwitch("toggle-auto-updates", state.enable_auto_updates)}</div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-icon">NAS</div>
            <div class="card-text">
              <div class="card-title">Automated weekly AppData backups</div>
              <div class="card-desc">Automatically creates compressed snapshots of /DATA/AppData in /DATA/Backups</div>
            </div>
          </div>
          <div class="card-right">${renderSwitch("toggle-scheduled-backups", state.enable_scheduled_backups)}</div>
        </div>

        ${state.ui_mode === "pro" ? `
          <div class="win-card">
            <div class="card-left">
              <div class="card-icon">DKR</div>
              <div class="card-text">
                <div class="card-title">Docker network range &amp; Prometheus metrics</div>
                <div class="card-desc">Custom Docker bridge network range and optional system metrics endpoint</div>
              </div>
            </div>
            <div class="card-right" style="flex-wrap:wrap;">
              <input class="win-input" style="min-width:150px;" data-field="docker_subnet_cidr" value="${esc(state.docker_subnet_cidr)}" placeholder="172.17.0.1/16" />
              ${renderSwitch("toggle-prometheus", state.enable_prometheus_metrics)}
            </div>
          </div>
        ` : ""}
      `;

      return `
        <div class="tab-flow-stack">
          ${basicHtml}
          <div class="appstore-grid" id="appStoreGridContainer">
            ${renderFilteredAppCardsHtml()}
          </div>
          ${renderExpander("apps", "Advanced options", advHtml + buildUniversalAppFileStudioHtml())}
        </div>
      `;
    }

    function buildStorageSection() {
      const modes = [
        { id: "dual_boot", icon: "BAL", title: "Dual-Boot alongside existing OS", desc: "Keeps your computer's original Windows or Linux system intact (15 GB – 64 TB)", badge: "Safe Default" },
        { id: "auto_erase", icon: "CLN", title: "Fresh Server Install (Erase main drive)", desc: "Formats the main internal drive and dedicates 100% of space to the server", badge: "Requires confirmation" },
        { id: "reinstall_keep_data", icon: "REC", title: "Reinstall OS while retaining server data (/DATA & apps preserved)", desc: "Backs up existing /DATA, photos, Nextcloud, and app configs before reinstalling Ubuntu LTS and restores them automatically", badge: "Preserves /DATA" },
        { id: "reinstall_wipe_data", icon: "", title: "Reinstall OS without retaining server data (Factory reset wipe)", desc: "Erases all existing server data, apps, and partitions for a clean factory reset", badge: "Requires WIPE confirmation" },
        { id: "offline_updater_usb", icon: "PKG", title: "Offline Updater USB (For local-only / air-gapped servers)", desc: "Creates an offline update USB stick that updates the Web Control Center, scripts, and app stacks without internet", badge: "Zero-Internet Update" },
        { id: "repair_rescue_usb", icon: "SSH", title: "Emergency Repair & Password Reset USB", desc: "Resets forgotten admin/SSH/dashboard password to the password set in Tab 1, repairs GRUB/EFI boot, and runs e2fsck without erasing files", badge: "Does not erase drive" },
        { id: "secondary_drive", icon: "DSK", title: "Secondary drive only", desc: "Installs onto a second internal drive and leaves the primary drive untouched", badge: "" }
      ];

      const displaySizeLabel = state.dual_boot_Allocation_gb >= 1024 && (state.dual_boot_Allocation_gb % 1024 === 0)
        ? `${state.dual_boot_Allocation_gb / 1024} TB (${state.dual_boot_Allocation_gb} GB)`
        : `${state.dual_boot_Allocation_gb} GB`;
      const sizePresets = [
        { gb: 64, label: "64 GB" },
        { gb: 250, label: "250 GB" },
        { gb: 500, label: "500 GB" },
        { gb: 1024, label: "1 TB" },
        { gb: 2048, label: "2 TB" },
        { gb: 4096, label: "4 TB" },
        { gb: 8192, label: "8 TB" },
        { gb: 16384, label: "16 TB" }
      ];
      const modeConfigCard = state.install_mode === "dual_boot" ? `
        <div class="win-card" style="flex-direction:column;align-items:stretch;gap:10px;">
          <div style="display:flex;align-items:center;justify-content:space-between;gap:14px;flex-wrap:wrap;">
            <div class="card-left">
              <div class="card-icon">LIM</div>
              <div class="card-text">
                <div class="card-title" id="lblDualBootSize">Server partition size (${esc(displaySizeLabel)})</div>
                <div class="card-desc" id="lblStorageValidation" style="${state.storageInputError ? "color:var(--danger);font-weight:600;" : ""}">
                  ${esc(state.storageInputError || "Type any size from 15 GB up to 64 TB (65,536 GB) or pick a preset below")}
                </div>
              </div>
            </div>
            <div class="storage-input-group">
              <input
                type="number"
                id="inputStorageAllocation"
                class="storage-num-input ${state.storageInputError ? "invalid" : ""}"
                min="1"
                max="65536"
                step="any"
                data-field="storage_custom_input"
                value="${esc(state.storageInputRaw)}"
                aria-label="Server partition size"
              />
              <div class="mode-segmented-switch" role="group" aria-label="Storage Unit">
                <button type="button" class="mode-seg-btn ${state.storageUnit === "GB" ? "active-standard" : ""}" data-action="set-storage-unit" data-unit="GB">GB</button>
                <button type="button" class="mode-seg-btn ${state.storageUnit === "TB" ? "active-standard" : ""}" data-action="set-storage-unit" data-unit="TB">TB</button>
              </div>
            </div>
          </div>
          <div class="storage-presets-row">
            ${sizePresets.map(p => `
              <button type="button" class="storage-preset-pill ${Number(state.dual_boot_Allocation_gb) === p.gb ? "active" : ""}" data-action="set-storage-preset" data-gb="${p.gb}">
                ${esc(p.label)}
              </button>
            `).join("")}
          </div>
        </div>
      ` : state.install_mode === "auto_erase" ? `
        <div class="notice-banner ${state.confirm_erase_main_drive ? "notice-banner-warning" : ""}" id="bannerAutoErase">
          <div style="font-weight:600;">Warning: This will erase all files on the target computer's main internal drive</div>
          <label style="display:flex;align-items:center;gap:8px;cursor:pointer;margin-top:4px;">
            <input type="checkbox" data-action="toggle-confirm-erase" ${state.confirm_erase_main_drive ? "checked" : ""} style="accent-color:var(--danger);width:16px;height:16px;" />
            <span>I know what I'm doing — erase the target computer's main internal drive</span>
          </label>
        </div>
      ` : state.install_mode === "reinstall_wipe_data" ? `
        <div class="notice-banner notice-banner-warning" id="bannerReinstallWipe">
          <div style="font-weight:600;color:var(--danger);">WARN Factory Reset Warning: This will permanently erase all existing server data (/DATA), containers, and backups</div>
          <label style="display:flex;align-items:center;gap:8px;cursor:pointer;margin-top:4px;">
            <input type="checkbox" data-action="toggle-confirm-wipe-server-data" ${state.confirm_wipe_server_data ? "checked" : ""} style="accent-color:var(--danger);width:16px;height:16px;" />
            <span>I confirm erasing all existing server data (WIPE-SERVER-DATA) and performing a clean factory reinstall</span>
          </label>
        </div>
      ` : state.install_mode === "reinstall_keep_data" ? `
        <div class="notice-banner" style="border-color:var(--success);">
          <div style="font-weight:600;">REC Safe OS Reinstall Mode Active</div>
          <div style="font-size:12px;">Existing <code>/DATA</code> (Photos, Nextcloud, Minecraft worlds, AppData) and <code>/opt/instant-server</code> are backed up prior to OS formatting and restored automatically after Ubuntu LTS finishes installing.</div>
        </div>
      ` : state.install_mode === "offline_updater_usb" ? `
        <div class="notice-banner" style="border-color:var(--accent);">
          <div style="font-weight:600;">PKG Offline Air-Gapped Updater USB Mode Active</div>
          <div style="font-size:12px;">Plug this USB into your running local server (or boot from it) to update the Web Dashboard, hardening scripts, and Docker Compose app stacks without needing internet access.</div>
        </div>
      ` : state.install_mode === "repair_rescue_usb" ? `
        <div class="notice-banner" style="border-color:var(--success);">
          <div style="font-weight:600;">SSH Emergency Repair &amp; Password Reset Mode Active</div>
          <div style="font-size:12px;">Booting from this USB automatically resets the password for <strong>${esc(state.username || "admin")}</strong> to the password entered in Tab 1, runs <code>e2fsck -fy</code> on ext4 partitions, and clears UFW/Fail2ban lockouts without erasing your data.</div>
        </div>
      ` : "";

      const usbListHtml = state.usbDevices.map(dev => {
        const selected = state.selectedUsbId === dev.device_id && dev.triple_lock_passed;
        return `
          <div class="win-card ${dev.triple_lock_passed ? "clickable" : ""} ${selected ? "selected-accent" : ""}"
               ${dev.triple_lock_passed ? `data-action="select-usb" data-usb="${esc(dev.device_id)}"` : ""}
               style="${!dev.triple_lock_passed ? "opacity:0.55;" : ""}">
            <div class="card-left">
              <div class="card-icon">${dev.triple_lock_passed ? "UPS" : "SEC"}</div>
              <div class="card-text">
                <div class="card-title">
                  ${esc(dev.model)} (${dev.size_gb} GB)
                  <span class="badge ${dev.triple_lock_passed ? "badge-success" : "badge-danger"}">
                    ${dev.triple_lock_passed ? "Ready" : "Protected System Drive"}
                  </span>
                </div>
                <div class="card-desc">${esc(dev.device_node)} ${dev.mountpoints && dev.mountpoints.length ? "(" + esc(dev.mountpoints.join(", ")) + ")" : ""} ${dev.block_reasons && dev.block_reasons.length ? "• " + esc(dev.block_reasons[0]) : ""}</div>
              </div>
            </div>
            <div class="card-right">
              ${dev.triple_lock_passed
                ? `<input type="radio" name="usb_sel" ${selected ? "checked" : ""} style="accent-color:var(--accent);width:17px;height:17px;pointer-events:none;" />`
                : `<span style="font-size:12px;color:var(--text-tertiary);">Blocked</span>`}
            </div>
          </div>
        `;
      }).join("");

      const advHtml = `
        <div class="win-card">
          <div class="card-left">
            <div class="card-icon">GPT</div>
            <div class="card-text">
              <div class="card-title">USB partition scheme &amp; target firmware</div>
              <div class="card-desc">Automatically set from your computer age answer above, or override manually</div>
            </div>
          </div>
          <div class="card-right">
            ${renderComboBox("usb_partition_scheme", [
              { value: "auto", label: "Universal Hybrid (UEFI + Legacy BIOS)" },
              { value: "gpt_uefi", label: "GPT Partition Scheme for UEFI" },
              { value: "mbr_bios", label: "MBR Partition Scheme for BIOS / CSM" }
            ])}
          </div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-icon">ISO</div>
            <div class="card-text">
              <div class="card-title">Ubuntu Server ISO source</div>
              <div class="card-desc">Pull official ISO automatically from Ubuntu, or use a downloaded .iso file on your PC</div>
            </div>
          </div>
          <div class="card-right" style="flex-wrap:wrap;">
            ${renderComboBox("iso_source_mode", [
              { value: "official_dl", label: "Auto-Download Official Ubuntu ISO" },
              { value: "local_file", label: "Use Local .iso File on This PC" }
            ])}
            <input
              id="customIsoPathInput"
              class="win-input"
              style="min-width:240px; ${state.iso_source_mode === "local_file" ? "" : "display:none;"}"
              data-field="custom_iso_path"
              value="${esc(state.custom_iso_path)}"
              placeholder="C:\\Downloads\\ubuntu-26.04.1-live-server-amd64.iso"
            />
          </div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-icon">LTS</div>
            <div class="card-text">
              <div class="card-title">Ubuntu Server LTS release</div>
              <div class="card-desc">Official Ubuntu Server LTS version</div>
            </div>
          </div>
          <div class="card-right">
            ${renderComboBox("ubuntu_release", [
              { value: "26.04", label: "Ubuntu Server 26.04.1 LTS (Default — Latest LTS)" },
              { value: "24.04", label: "Ubuntu Server 24.04.5 LTS" },
              { value: "22.04", label: "Ubuntu Server 22.04.5 LTS" }
            ])}
          </div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-icon">EXT4</div>
            <div class="card-text">
              <div class="card-title">Target filesystem layout</div>
              <div class="card-desc">Partition structure on the hard drive</div>
            </div>
          </div>
          <div class="card-right">
            ${renderComboBox("storage_layout", [
              { value: "direct", label: "Standard ext4 (Recommended)" },
              { value: "lvm", label: "LVM Volume Group" },
              { value: "btrfs", label: "Btrfs Snapshots" }
            ])}
          </div>
        </div>
      `;

      return `
        <div class="tab-flow-stack">
          <div class="section-label">1. About the target computer (If you don't know, leave on "Auto-detect")</div>
          <div class="card-stack">
            <div class="win-card">
              <div class="card-left">
                <div class="card-icon">PC</div>
                <div class="card-text">
                  <div class="card-title">What type of computer is this?</div>
                  <div class="card-desc">Automatically enables battery &amp; lid care on laptops, or low-power desktop settings on Mini-PCs and Desktops</div>
                </div>
              </div>
              <div class="card-right">
                ${renderComboBox("device_form_factor", [
                  { value: "auto_detect", label: "Auto-detect (Laptop, Desktop, or Mini-PC)" },
                  { value: "laptop_ewaste", label: "Old laptop (Battery & lid protection + low-RAM boost)" },
                  { value: "laptop_modern", label: "Modern or gaming laptop" },
                  { value: "mini_pc_n100", label: "Mini-PC / Intel N100 (Ultra-low-power desktop mode)" },
                  { value: "desktop_pc", label: "Desktop PC or Tower (No battery settings needed)" },
                  { value: "nas_multibay", label: "Multi-drive NAS or Workstation" }
                ])}
              </div>
            </div>

            <div class="win-card">
              <div class="card-left">
                <div class="card-icon">BOOT</div>
                <div class="card-text">
                  <div class="card-title">How old is the computer?</div>
                  <div class="card-desc">Selects the right boot loader (UEFI GPT vs Legacy BIOS MBR). "I don't know" creates a universal hybrid boot USB.</div>
                </div>
              </div>
              <div class="card-right">
                ${renderComboBox("laptop_age_hint", [
                  { value: "unknown", label: "I don't know (Universal Hybrid Boot)" },
                  { value: "modern_uefi", label: "2013 or newer (UEFI + GPT)" },
                  { value: "legacy_bios", label: "2012 or older (Legacy BIOS + MBR)" }
                ])}
              </div>
            </div>

            <div class="win-card">
              <div class="card-left">
                <div class="card-icon">RAM</div>
                <div class="card-text">
                  <div class="card-title">How much RAM does it have?</div>
                  <div class="card-desc">Tunes memory compression (zram) so even 2 GB – 4 GB computers run containers smoothly</div>
                </div>
              </div>
              <div class="card-right">
                ${renderComboBox("laptop_ram_hint", [
                  { value: "unknown", label: "I don't know (Auto-tune memory)" },
                  { value: "low_2_4gb", label: "2 GB – 4 GB RAM (Low-RAM zram boost)" },
                  { value: "standard_8gb_plus", label: "8 GB RAM (Standard Homelab & Minecraft)" },
                  { value: "high_end_16gb_gpu", label: "16 GB+ RAM / GPU (Higher-End — Local AI & Gaming)" }
                ])}
              </div>
            </div>
          </div>

          <div class="section-label">2. Target hard drive installation mode</div>
          <div class="card-stack">
            ${modes.map(m => {
              const active = state.install_mode === m.id;
              return `
                <div class="win-card clickable ${active ? "selected-accent" : ""}" data-action="select-install-mode" data-value="${m.id}">
                  <div class="card-left">
                    <div class="card-icon">${m.icon}</div>
                    <div class="card-text">
                      <div class="card-title">
                        ${esc(m.title)}
                        ${m.badge ? `<span class="badge ${m.id === "auto_erase" ? "badge-warning" : m.id === "repair_rescue_usb" ? "badge-success" : "badge-accent"}">${esc(m.badge)}</span>` : ""}
                      </div>
                      <div class="card-desc">${esc(m.desc)}</div>
                    </div>
                  </div>
                  <div class="card-right">
                    <input type="radio" name="inst_mode" ${active ? "checked" : ""} style="accent-color:var(--accent);width:17px;height:17px;pointer-events:none;" />
                  </div>
                </div>
              `;
            }).join("")}
            ${modeConfigCard}
          </div>

          <div style="display:flex;align-items:center;justify-content:space-between;gap:8px;flex-wrap:wrap;">
            <div class="section-label" style="margin:0;">3. Select USB drive to format &amp; make bootable</div>
            <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap;">
              <button type="button" class="win-btn" data-action="reset-usb-drive" ${state.resettingUsb || state.flashing ? "disabled" : ""} title="Clear USB write-protection and format as a normal full-capacity FAT32 drive">
                ${state.resettingUsb ? "Resetting USB..." : "Reset USB to Normal Drive"}
              </button>
              <button type="button" class="win-btn" data-action="scan-usb" ${state.scanningUsb || state.flashing ? "disabled" : ""}>
                ${state.scanningUsb ? "Scanning..." : "Refresh Drives"}
              </button>
            </div>
          </div>
          <div class="card-stack">
            ${usbListHtml}
          </div>

          <div class="notice-banner notice-banner-warning" id="bannerUsbFormatWarn">
            <div style="font-weight:600;">USB Format Warning: All existing files on the selected USB stick will be erased</div>
            <label style="display:flex;align-items:center;gap:8px;cursor:pointer;margin-top:2px;">
              <input type="checkbox" data-action="toggle-confirm-usb-erase" ${state.confirm_usb_erase ? "checked" : ""} style="accent-color:var(--accent);width:16px;height:16px;" />
              <span>Format the selected USB drive and write the bootable Ubuntu Server ISO + pre-selected apps</span>
            </label>
          </div>

          
          <div class="win-card" style="flex-direction:column;align-items:stretch;gap:10px;">
            <div style="display:flex;align-items:center;justify-content:space-between;gap:10px;flex-wrap:wrap;">
              <div class="card-left">
                <div class="card-icon">DOC</div>
                <div class="card-text">
                  <div class="card-title">How to Boot From USB &amp; Reset Forgotten Passwords (Quick Guide)</div>
                  <div class="card-desc">Press the Boot Menu key repeatedly right after powering on the target laptop/PC • Sleep is automatically inhibited during install</div>
                </div>
              </div>
              <span class="badge badge-accent">No Monitor Needed After Boot Selection</span>
            </div>
            <div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(210px,1fr));gap:8px;font-size:12px;">
              <div style="padding:8px 10px;border-radius:8px;background:var(--card-bg-secondary);border:1px solid var(--border-subtle);">
                <strong>Lenovo / ThinkPad / Dell / Acer:</strong> Tap <code>F12</code> repeatedly at power-on (BIOS Setup: <code>F2</code> or <code>F1</code>)
              </div>
              <div style="padding:8px 10px;border-radius:8px;background:var(--card-bg-secondary);border:1px solid var(--border-subtle);">
                <strong>HP (Pavilion / EliteBook / Omen):</strong> Tap <code>F9</code> or <code>Esc → F9</code> at power-on (BIOS Setup: <code>F10</code>)
              </div>
              <div style="padding:8px 10px;border-radius:8px;background:var(--card-bg-secondary);border:1px solid var(--border-subtle);">
                <strong>ASUS / MSI / Desktop Boards:</strong> Tap <code>F8</code> or <code>Esc</code> (ASUS), <code>F11</code> (MSI), <code>F12</code> (Gigabyte)
              </div>
              <div style="padding:8px 10px;border-radius:8px;background:var(--card-bg-secondary);border:1px solid var(--border-subtle);">
                <strong>Forgot Server Password?</strong> Pick <em>Emergency Repair &amp; Password Reset USB</em> above, set a new password in Tab 1, flash, and boot once
              </div>
            </div>
          </div>

          ${renderExpander("storage", "Advanced options", advHtml)}
        </div>
      `;
    }

    function buildPowerSection() {
      const basicHtml = `
        <div class="card-stack">
          <div class="win-card">
            <div class="card-left">
              <div class="card-icon">SYS</div>
              <div class="card-text">
                <div class="card-title">Keep running with the laptop lid closed</div>
                <div class="card-desc">Prevents the laptop from going to sleep when you close the screen (automatically skipped on desktop PCs)</div>
              </div>
            </div>
            <div class="card-right">${renderSwitch("toggle-lid", state.ignore_lid_switch)}</div>
          </div>

          <div class="notice-banner notice-banner-warning" id="bannerThermalOverheatTrap" style="${state.ignore_lid_switch ? "" : "display:none;"}">
            <div style="display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;">
              <div style="font-weight:600;">WARN Keep laptop in an open, ventilated area</div>
              <button type="button" class="win-btn win-btn-sm ${state.disable_cpu_turbo_boost ? "win-btn-accent" : ""}" data-action="enable-cool-quiet-profile">
                ${state.disable_cpu_turbo_boost ? " Cool & Quiet Mode ON" : "Enable Cool & Quiet Mode"}
              </button>
            </div>
            <div style="font-size:12px;line-height:1.45;color:var(--text-primary);">
              Avoid placing a closed laptop inside enclosed drawers or cabinets. Keep airflow unobstructed or prop the lid slightly ajar.
            </div>
          </div>

          <div class="win-card">
            <div class="card-left">
              <div class="card-icon">PWR</div>
              <div class="card-text">
                <div class="card-title" id="lblBatCapTitle">Battery health limiter (${state.battery_charge_start_pct}% – ${state.battery_charge_stop_pct}%)</div>
                <div class="card-desc" id="lblBatCapDesc">Caps charging at ${state.battery_charge_stop_pct}% while plugged in 24/7 to protect the battery</div>
              </div>
            </div>
            <div class="card-right">
              ${renderSlider("battery_charge_stop_pct", 50, 85, 5, state.battery_charge_stop_pct, "%", 160)}
            </div>
          </div>

          <div class="win-card">
            <div class="card-left">
              <div class="card-icon">FAST</div>
              <div class="card-text">
                <div class="card-title" id="lblUpsTitle">Safe shutdown during power outage (${state.ups_shutdown_battery_pct}%)</div>
                <div class="card-desc" id="lblUpsDesc">Uses the laptop battery as a built-in UPS and shuts down cleanly when battery drops to ${state.ups_shutdown_battery_pct}%</div>
              </div>
            </div>
            <div class="card-right">
              ${renderSlider("ups_shutdown_battery_pct", 10, 35, 5, state.ups_shutdown_battery_pct, "%", 160)}
            </div>
          </div>
        </div>
      `;

      const advHtml = `
        <div class="win-card">
          <div class="card-left">
            <div class="card-icon">PC</div>
            <div class="card-text">
              <div class="card-title" id="lblScreenBlankTitle">Turn off screen after ${esc(state.screen_blank_seconds)} seconds</div>
              <div class="card-desc">Saves power and prevents screen burn-in</div>
            </div>
          </div>
          <div class="card-right">
            ${renderComboBox("screen_blank_seconds", [
              { value: "30", label: "30 seconds" },
              { value: "60", label: "60 seconds (Recommended)" },
              { value: "180", label: "3 minutes" }
            ])}
          </div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-icon">RAM</div>
            <div class="card-text">
              <div class="card-title">Low-RAM memory compression</div>
              <div class="card-desc">Enables zram swap so older 2 GB and 4 GB RAM computers run smoothly</div>
            </div>
          </div>
          <div class="card-right">${renderSwitch("toggle-zram", state.enable_low_ram_zram)}</div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-icon">MC</div>
            <div class="card-text">
              <div class="card-title">Safe graphics mode</div>
              <div class="card-desc">Use if the computer has an older NVIDIA or AMD graphics chip</div>
            </div>
          </div>
          <div class="card-right">${renderSwitch("toggle-nomodeset", state.safe_gpu_nomodeset)}</div>
        </div>

        ${state.ui_mode === "pro" ? `
          <div class="win-card">
            <div class="card-left">
              <div class="card-icon">CFG</div>
              <div class="card-text">
                <div class="card-title">CPU power &amp; fan profile</div>
                <div class="card-desc">Select Quiet &amp; Low Power for silent fans, or Balanced for automatic speed boost</div>
              </div>
            </div>
            <div class="card-right">
              ${renderComboBox("cpu_governor", [
                { value: "schedutil", label: "Balanced (Default)" },
                { value: "powersave", label: "Quiet & Low Power" },
                { value: "performance", label: "Maximum Speed" }
              ])}
            </div>
          </div>

          <div class="win-card">
            <div class="card-left">
              <div class="card-icon">CPU</div>
              <div class="card-text">
                <div class="card-title">Limit peak CPU heat (Disable Turbo Boost)</div>
                <div class="card-desc">Lowers peak CPU temperatures and fan noise on older computers</div>
              </div>
            </div>
            <div class="card-right">${renderSwitch("toggle-disable-turbo", state.disable_cpu_turbo_boost)}</div>
          </div>

          <div class="win-card">
            <div class="card-left">
              <div class="card-icon">SIM</div>
              <div class="card-text">
                <div class="card-title" id="lblSwappinessTitle">Memory swap tendency (${state.vm_swappiness}%)</div>
                <div class="card-desc">Keeps apps in physical RAM longer and reduces drive wear</div>
              </div>
            </div>
            <div class="card-right">
              ${renderSlider("vm_swappiness", 1, 60, 1, state.vm_swappiness, "%", 140)}
            </div>
          </div>
        ` : ""}
      `;

      return `
        <div class="tab-flow-stack">
          ${basicHtml}
          ${renderExpander("power", "Advanced options", advHtml)}
        </div>
      `;
    }

    function renderCacheRetentionCardHtml() {
      return `
        <div class="win-card" style="flex-direction:column;align-items:stretch;gap:10px;border-color:var(--success);">
          <div style="display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;">
            <div class="card-left">
              <div class="card-icon">FS</div>
              <div class="card-text">
                <div class="card-title">
                  <span>Cached ISO &amp; App Files</span>
                  <span class="badge badge-success">Kept by Default</span>
                </div>
                <div class="card-desc">Keep downloaded ISO and app files in <code>Instant_Server_Data</code> for fast reflashing, or delete them to free disk space.</div>
              </div>
            </div>
            <div class="card-right" style="gap:8px;flex-wrap:wrap;">
              <button type="button" class="win-btn win-btn-sm ${state.cacheRetentionMode !== "delete" ? "win-btn-accent" : ""}" data-action="cache-keep">
                Keep Cached Files
              </button>
              <button type="button" class="win-btn win-btn-sm ${state.cacheRetentionMode === "delete" ? "win-btn-danger" : ""}" data-action="cache-delete">
                Delete Cached Files
              </button>
            </div>
          </div>
          <div style="font-size:12px;color:var(--text-secondary);">
            Status: <strong>${esc(state.cacheStatusMessage)}</strong>
          </div>
        </div>
      `;
    }

    function buildFinishSection() {
      const sum = buildSummaryLabels();
      const selectedTitles = APP_STORE_CATALOG.filter(a => state.selected_apps.includes(a.id)).map(a => a.title);
      return `
        <div class="tab-flow-stack">
          <div class="card-stack">
            <div class="win-card">
              <div class="card-left">
                <div class="card-icon">SYS</div>
                <div class="card-text">
                  <div class="card-title">${esc(state.hostname || "instant-server")}.local</div>
                  <div class="card-desc">Ubuntu Server ${esc(state.ubuntu_release)} LTS • Admin user: ${esc(state.username || "admin")}</div>
                </div>
              </div>
              <div class="card-right">
                <span class="badge ${state.password ? "badge-success" : "badge-warning"}">
                  ${state.password ? "Password set" : "Password empty"}
                </span>
              </div>
            </div>

            <div class="win-card">
              <div class="card-left">
                <div class="card-icon">NET</div>
                <div class="card-text">
                  <div class="card-title">Network</div>
                  <div class="card-desc">${esc(sum.wifiLabel)}${state.cluster_mode !== "standalone" ? " • Multi-computer cluster enabled" : ""}</div>
                </div>
              </div>
              <div class="card-right">
                <button type="button" class="win-btn" data-action="jump-setup-tab" data-tab="network">Edit</button>
              </div>
            </div>

            <div class="win-card">
              <div class="card-left">
                <div class="card-icon">APP</div>
                <div class="card-text">
                  <div class="card-title">Dashboards &amp; ${selectedTitles.length} pre-selected apps</div>
                  <div class="card-desc">${esc(selectedTitles.join(", ") || "None selected")}</div>
                </div>
              </div>
              <div class="card-right">
                <button type="button" class="win-btn" data-action="jump-setup-tab" data-tab="apps">Edit</button>
              </div>
            </div>

            <div class="win-card">
              <div class="card-left">
                <div class="card-icon">USB</div>
                <div class="card-text">
                  <div class="card-title">Bootable USB &amp; target installation mode</div>
                  <div class="card-desc">${esc(sum.modeLabel)} • ${esc(sum.usbLabel)} • Scheme: ${esc(state.usb_partition_scheme.toUpperCase())}</div>
                </div>
              </div>
              <div class="card-right" style="gap:8px;">
                <button type="button" class="win-btn" data-action="reset-usb-drive" ${state.resettingUsb || state.flashing ? "disabled" : ""}>
                  ${state.resettingUsb ? "Resetting USB..." : "Reset USB"}
                </button>
                <button type="button" class="win-btn" data-action="jump-setup-tab" data-tab="storage">Edit</button>
              </div>
            </div>
          </div>

          ${renderCacheRetentionCardHtml()}

          <div class="win-card" style="flex-direction:column;align-items:stretch;gap:8px;">
            <div class="card-left">
              <div class="card-icon">WEB</div>
              <div class="card-text">
                <div class="card-title">Next steps after flashing</div>
                <div class="card-desc">
                  <strong>1. Boot from USB:</strong> Plug the USB into your target computer, power it on, and select the USB drive from the boot menu (F12 / F9 / F8 / F11).<br/>
                  <strong>2. Automatic setup:</strong> Ubuntu Server and your selected apps install automatically.<br/>
                  <strong>3. Open Web Dashboard:</strong> Once complete, the screen displays your Web Dashboard URL (<code>http://${esc(state.hostname || "instant-server")}.local:${state.dashboard_port}</code>).
                </div>
              </div>
            </div>
          </div>

          ${renderFlashProgressAndLogsCard()}
        </div>
      `;
    }

    function renderSetupView() {
      const topTabsEl = document.getElementById("setupTopTabs");
      const bodyEl = document.getElementById("setupBodyContent");
      const footerEl = document.getElementById("setupFooterBar");

      topTabsEl.innerHTML = SETUP_TABS.map((t, idx) => {
        const active = state.setupTab === t.id;
        return `
          <button type="button" class="setup-tab-btn ${active ? "active" : ""}" data-action="switch-setup-tab" data-tab="${t.id}">
            <span class="setup-tab-step-dot">${idx + 1}</span>
            <span>${esc(t.label)}</span>
          </button>
        `;
      }).join("");

      const tabIdx = SETUP_TABS.findIndex(t => t.id === state.setupTab);
      const headers = {
        network: { title: "Connect your server", desc: "Name your server and optionally enter your home Wi-Fi details." },
        apps: { title: "Choose dashboards & self-hosted apps", desc: "Pick your web dashboard and check the self-hosted apps you want pre-bundled onto the USB drive." },
        storage: { title: "Target computer details & bootable USB", desc: "Choose your computer type and installation mode (or Repair USB mode) and select your USB stick." },
        power: { title: "Power & battery settings", desc: "Configure lid-close behavior and battery protection for 24/7 operation." },
        finish: { title: "Ready to format & flash bootable USB", desc: "Pulls the official Ubuntu Server ISO, formats the USB drive, and bundles your config and selected apps." }
      };
      const hdr = headers[state.setupTab] || headers.network;

      let contentHtml = "";
      if (state.setupTab === "network") contentHtml = buildNetworkSection();
      else if (state.setupTab === "apps") contentHtml = buildAppsSection();
      else if (state.setupTab === "storage") contentHtml = buildStorageSection();
      else if (state.setupTab === "power") contentHtml = buildPowerSection();
      else contentHtml = buildFinishSection();

      const animClass = shouldAnimateTab ? "animate-tab-switch" : "";
      shouldAnimateTab = false;

      bodyEl.innerHTML = `
        <div class="${animClass} tab-flow-stack">
          <div>
            <h1 class="setup-header-title">${esc(hdr.title)}</h1>
            <p class="setup-header-desc">${esc(hdr.desc)}</p>
          </div>
          ${contentHtml}
        </div>
      `;

      const isFirst = tabIdx <= 0;
      const isLast = tabIdx === SETUP_TABS.length - 1;

      footerEl.innerHTML = `
        <div>
          <button type="button" class="win-btn" data-action="setup-prev" ${isFirst ? "disabled" : ""}>← Previous</button>
        </div>
        <div class="setup-footer-right">
          ${!isLast ? `
            <button type="button" class="win-btn" data-action="setup-skip">Skip</button>
            <button type="button" class="win-btn win-btn-accent" data-action="setup-next">Proceed →</button>
          ` : `
            <button type="button" class="win-btn" data-action="save-config-home">Save configuration</button>
            <span class="footer-cancel-flash-wrap" style="display:${state.flashing ? "inline-flex" : "none"};gap:8px;">
              <button type="button" class="win-btn" data-action="pause-resume-flash">${state.flashPaused ? `Resume (${state.flashProgress}%)` : "Pause"}</button>
              <button type="button" class="win-btn win-btn-danger" data-action="cancel-flash">Cancel</button>
            </span>
            <button type="button" class="win-btn win-btn-accent win-btn-large" data-action="start-flash" ${state.flashing ? "disabled" : ""}>
              ${state.flashing ? "Flashing Bootable USB..." : "Confirm & Flash Bootable USB"}
            </button>
          `}
        </div>
      `;
    }

    function renderSettingsView() {
      const navEl = document.getElementById("settingsNavList");
      const mainEl = document.getElementById("settingsMainContent");

      navEl.innerHTML = SETTINGS_TABS.map(item => {
        const active = state.settingsTab === item.id && !state.searchQuery.trim();
        return `
          <button type="button" class="nav-item ${active ? "active" : ""}" data-action="switch-settings-tab" data-tab="${item.id}">
            <span class="nav-icon">${item.icon}</span>
            <span>${esc(item.label)}</span>
          </button>
        `;
      }).join("");

      if (state.searchQuery.trim()) {
        mainEl.innerHTML = renderSearchResults(state.searchQuery.trim());
        return;
      }

      const tabIdx = SETTINGS_TABS.findIndex(t => t.id === state.settingsTab);
      const currentMeta = SETTINGS_TABS[tabIdx] || SETTINGS_TABS[0];
      const isFirst = tabIdx <= 0;
      const isLast = tabIdx === SETTINGS_TABS.length - 1;

      const animClass = shouldAnimateTab ? "animate-tab-switch" : "";
      shouldAnimateTab = false;

      const bottomPagerHtml = `
        <div class="bottom-nav-row">
          <button type="button" class="win-btn" data-action="settings-prev" ${isFirst ? "disabled" : ""}>← Previous</button>
          <div style="display:flex;align-items:center;gap:10px;">
            <button type="button" class="win-btn" data-action="save-config-home">Save configuration</button>
            ${isLast ? `
              <button type="button" class="win-btn win-btn-accent win-btn-large" data-action="start-flash" ${state.flashing ? "disabled" : ""}>
                ${state.flashing ? "Flashing Bootable USB..." : "Flash Bootable USB"}
              </button>
            ` : `
              <button type="button" class="win-btn win-btn-accent" data-action="settings-next">Next →</button>
            `}
          </div>
        </div>
      `;

      if (state.settingsTab === "system") {
        const sum = buildSummaryLabels();
        const selectedTitles = APP_STORE_CATALOG.filter(a => state.selected_apps.includes(a.id)).map(a => a.title);

        mainEl.innerHTML = `
          <div class="${animClass} tab-flow-stack">
            <div class="page-header">
              <div class="breadcrumb-bar"><span>System</span></div>
            </div>

            <div class="system-summary-banner">
              <div class="summary-left">
                <div class="summary-title">${esc(state.hostname || "instant-server")}</div>
                <div class="summary-subtitle">Ubuntu Server ${esc(state.ubuntu_release)} LTS • Admin: ${esc(state.username || "admin")}</div>
              </div>
              <div class="summary-pills">
                <div class="summary-pill">Network: <strong>${esc(sum.wifiLabel)}</strong></div>
                <div class="summary-pill">Apps: <strong>${esc(sum.dashLabel)}</strong></div>
                <div class="summary-pill">Mode: <strong>${esc(sum.modeLabel)}</strong></div>
                <div class="summary-pill">USB: <strong>${esc(sum.usbLabel)}</strong></div>
              </div>
            </div>

            <div class="card-stack">
              <div class="win-card clickable" data-action="switch-settings-tab" data-tab="network">
                <div class="card-left">
                  <div class="card-icon">NET</div>
                  <div class="card-text">
                    <div class="card-title">Network &amp; multi-server</div>
                    <div class="card-desc">Server name (${esc(state.hostname || "instant-server")}), admin login, Wi-Fi (${esc(sum.wifiLabel)}), combine multiple computers</div>
                  </div>
                </div>
                <div class="card-right"><span>›</span></div>
              </div>

              <div class="win-card clickable" data-action="switch-settings-tab" data-tab="apps">
                <div class="card-left">
                  <div class="card-icon">APP</div>
                  <div class="card-text">
                    <div class="card-title">Apps &amp; store (${selectedTitles.length} selected)</div>
                    <div class="card-desc">${esc(selectedTitles.join(", ") || "No extra apps selected")}</div>
                  </div>
                </div>
                <div class="card-right"><span>›</span></div>
              </div>

              <div class="win-card clickable" data-action="switch-settings-tab" data-tab="storage">
                <div class="card-left">
                  <div class="card-icon">USB</div>
                  <div class="card-text">
                    <div class="card-title">Laptop, PC &amp; USB flash</div>
                    <div class="card-desc">${esc(sum.modeLabel)} • Target USB: ${esc(sum.usbLabel)} • Boot scheme: ${esc(state.usb_partition_scheme.toUpperCase())}</div>
                  </div>
                </div>
                <div class="card-right"><span>›</span></div>
              </div>

              <div class="win-card clickable" data-action="switch-settings-tab" data-tab="power">
                <div class="card-left">
                  <div class="card-icon">PWR</div>
                  <div class="card-text">
                    <div class="card-title">Power &amp; battery</div>
                    <div class="card-desc">Lid-close sleep ignore (${state.ignore_lid_switch ? "On" : "Off"}), battery charge cap (${state.battery_charge_stop_pct}%), outage auto-shutdown (${state.ups_shutdown_battery_pct}%)</div>
                  </div>
                </div>
                <div class="card-right"><span>›</span></div>
              </div>
            </div>

            <div class="center-flash-row" style="flex-direction:row;flex-wrap:wrap;gap:10px;">
              <button type="button" class="win-btn win-btn-accent win-btn-large" data-action="start-flash" ${state.flashing ? "disabled" : ""}>
                ${state.flashing ? "Flashing Bootable USB..." : "Flash Bootable USB"}
              </button>
              <button type="button" class="win-btn win-btn-large" data-action="reset-usb-drive" ${state.resettingUsb || state.flashing ? "disabled" : ""}>
                ${state.resettingUsb ? "Resetting USB..." : "Reset USB to Normal Drive"}
              </button>
              <span class="footer-cancel-flash-wrap" style="display:${state.flashing ? "inline-flex" : "none"};gap:8px;">
                <button type="button" class="win-btn win-btn-large" data-action="pause-resume-flash">${state.flashPaused ? `Resume (${state.flashProgress}%)` : "Pause"}</button>
                <button type="button" class="win-btn win-btn-danger win-btn-large" data-action="cancel-flash">Cancel</button>
              </span>
            </div>

            ${renderCacheRetentionCardHtml()}
            ${renderFlashProgressAndLogsCard()}
            ${bottomPagerHtml}
          </div>
        `;
        return;
      }

      let bodyHtml = "";
      if (state.settingsTab === "network") bodyHtml = buildNetworkSection();
      else if (state.settingsTab === "apps") bodyHtml = buildAppsSection();
      else if (state.settingsTab === "storage") bodyHtml = buildStorageSection();
      else if (state.settingsTab === "power") bodyHtml = buildPowerSection();
      else if (state.settingsTab === "config") {
        bodyHtml = `
          <div class="tab-flow-stack">
            <div class="card-stack">
              <div class="win-card">
                <div class="card-left">
                  <div class="card-icon">FS</div>
                  <div class="card-text">
                    <div class="card-title">Local configuration &amp; workspace folder</div>
                    <div class="card-desc">Automatically saved in ${esc(state.savedConfigPath)}</div>
                  </div>
                </div>
                <div class="card-right">
                  <button type="button" class="win-btn" data-action="save-config-home">Save configuration</button>
                  <button type="button" class="win-btn" data-action="copy-yaml">Copy YAML</button>
                </div>
              </div>

              ${renderCacheRetentionCardHtml()}

              <div class="win-card">
                <div class="card-left">
                  <div class="card-icon">LAN</div>
                  <div class="card-text">
                    <div class="card-title">Find Instant Server on your network</div>
                    <div class="card-desc">Scan your home network for running Instant Server computers</div>
                  </div>
                </div>
                <div class="card-right">
                  <button type="button" class="win-btn" data-action="scan-lan">${state.lanScanning ? "Scanning..." : "Scan network"}</button>
                </div>
              </div>
            </div>

            ${state.lanScanResults.length > 0 ? `
              <div class="card-stack">
                ${state.lanScanResults.map(s => `
                  <div class="win-card">
                    <div class="card-left">
                      <div class="card-icon">ON</div>
                      <div class="card-text">
                        <div class="card-title">${esc(s.ip)}</div>
                        <div class="card-desc">Open ports: ${esc((s.open_ports || []).join(", "))}</div>
                      </div>
                    </div>
                  </div>
                `).join("")}
              </div>
            ` : ""}

            <div class="section-label">Common questions &amp; quick fixes</div>
            <div class="card-stack">
              ${QUICK_HELP_ITEMS.map(item => `
                <div class="win-card">
                  <div class="card-left">
                    <div class="card-icon">INFO</div>
                    <div class="card-text">
                      <div class="card-title">${esc(item.title)}</div>
                      <div class="card-desc">${esc(item.desc)}</div>
                    </div>
                  </div>
                  <div class="card-right">
                    <button type="button" class="win-btn win-btn-sm" data-action="${esc(item.action)}" ${item.tab ? `data-tab="${esc(item.tab)}"` : ""}>${esc(item.btn)}</button>
                  </div>
                </div>
              `).join("")}
            </div>

          </div>
        `;
      }

      mainEl.innerHTML = `
        <div class="${animClass} tab-flow-stack">
          <div class="page-header">
            <div class="breadcrumb-bar">
              <span class="breadcrumb-link" data-action="switch-settings-tab" data-tab="system">System</span>
              <span class="breadcrumb-sep">›</span>
              <span>${esc(currentMeta.label)}</span>
            </div>
          </div>
          ${bodyHtml}
          ${renderFlashProgressAndLogsCard()}
          ${bottomPagerHtml}
        </div>
      `;
    }

    function renderSearchResults(query) {
      const q = query.toLowerCase();
      const items = [
        { tab: "network", title: "Server name & admin login", desc: "Change hostname, admin username, and password", keys: "hostname password username login" },
        { tab: "network", title: "Combine multiple computers into one server", desc: "Use the same USB on multiple laptops/PCs or merge storage without losing files", keys: "combine multiple servers cluster mergerfs pool" },
        { tab: "network", title: "Wi-Fi & USB tethering", desc: "Configure wireless SSID, password, and phone USB tethering", keys: "wifi wireless ssid tethering static ip ssh" },
        { tab: "apps", title: "Self-Hosted App Store & CasaOS", desc: "Pre-select Jellyfin, Nextcloud, Pi-hole, Immich, Home Assistant, Vaultwarden, and more", keys: "cockpit casaos docker jellyfin nextcloud pihole immich homeassistant vaultwarden plex syncthing qbittorrent portainer" },
        { tab: "storage", title: "Computer type, Repair USB & Bootable Flash", desc: "Configure Laptop/Desktop/Mini-PC mode, Repair USB mode, and Dual-Boot size", keys: "dual boot erase disk drive storage partition usb flash iso repair rescue desktop mini pc" },
        { tab: "power", title: "Lid close & battery limiter", desc: "Keep running with lid closed and cap battery charge at 60%", keys: "power lid battery ups zram screen blank" },
        { tab: "config", title: "Saved configuration, LAN scanner & quick fixes", desc: "Save instant-server-config.json, scan LAN, or fix common issues", keys: "yaml config save lan discover help fix" }
      ].filter(it => (it.title + " " + it.desc + " " + it.keys).toLowerCase().includes(q));

      return `
        <div class="tab-flow-stack">
          <div class="page-header">
            <div class="breadcrumb-bar"><span>Search results for "${esc(query)}"</span></div>
            <button type="button" class="win-btn" data-action="clear-search">Clear</button>
          </div>
          <div class="card-stack">
            ${items.length ? items.map(it => `
              <div class="win-card clickable" data-action="switch-settings-tab" data-tab="${it.tab}">
                <div class="card-left">
                  <div class="card-icon">⌕</div>
                  <div class="card-text">
                    <div class="card-title">${esc(it.title)}</div>
                    <div class="card-desc">${esc(it.desc)}</div>
                  </div>
                </div>
                <div class="card-right"><span>Open ›</span></div>
              </div>
            `).join("") : `<div class="win-card"><div class="card-title">No matching settings found.</div></div>`}
          </div>
        </div>
      `;
    }

    function renderApp() {
      syncFlashingLockState();
      document.documentElement.setAttribute("data-theme", state.theme);
      const stdBtn = document.getElementById("btnUiModeStandard");
      const proBtn = document.getElementById("btnUiModePro");
      if (stdBtn && proBtn) {
        stdBtn.className = "mode-seg-btn " + (state.ui_mode === "standard" ? "active-standard" : "");
        proBtn.className = "mode-seg-btn " + (state.ui_mode === "pro" ? "active-pro" : "");
      }
      const themeLabel = document.getElementById("themeToggleLabel");
      if (themeLabel) {
        themeLabel.textContent = state.theme === "dark" ? "Light" : "Dark";
      }

      const modeBtn = document.getElementById("btnToggleModeView");
      if (modeBtn) {
        modeBtn.textContent = state.viewMode === "setup" ? "Home Settings" : "Setup Wizard";
      }

      const searchWrap = document.getElementById("titleSearchWrap");
      const setupView = document.getElementById("setupShellView");
      const settingsView = document.getElementById("settingsShellView");

      if (state.viewMode === "setup") {
        searchWrap.classList.add("hidden-in-setup");
        setupView.style.display = "flex";
        settingsView.style.display = "none";
        renderSetupView();
      } else {
        searchWrap.classList.remove("hidden-in-setup");
        setupView.style.display = "none";
        settingsView.style.display = "flex";
        renderSettingsView();
      }
    }

    function updateSwitchInPlace(switchEl, isOn) {
      if (!switchEl) return;
      switchEl.classList.toggle("on", isOn);
      switchEl.setAttribute("aria-checked", String(isOn));
      const lbl = switchEl.querySelector(".win-switch-label");
      if (lbl) lbl.textContent = isOn ? "On" : "Off";
    }

    function syncSelectAllCheckboxInPlace() {
      const countLbl = document.getElementById("lblSelectedAppsCount");
      if (countLbl) countLbl.textContent = String(state.selected_apps.length);
      const mcCard = document.getElementById("minecraftCustomizationStudioCard");
      if (mcCard) {
        mcCard.style.display = (state.selected_apps.includes("minecraft") || state.selected_apps.includes("minecraft_bedrock")) ? "flex" : "none";
      }
      const chk = document.getElementById("chkSelectAllApps");
      const txt = document.getElementById("lblSelectAllAppsText");
      const filtered = getFilteredAppsList();
      const targetList = filtered.length > 0 ? filtered : APP_STORE_CATALOG;
      const selectedInTarget = targetList.filter(a => state.selected_apps.includes(a.id)).length;
      const allSelected = targetList.length > 0 && selectedInTarget === targetList.length;
      if (chk) {
        chk.checked = allSelected;
        chk.indeterminate = selectedInTarget > 0 && !allSelected;
      }
      if (txt) {
        txt.textContent = allSelected
          ? (state.appCategoryFilter === "All" ? `Unselect all (${state.selected_apps.length})` : `Unselect ${state.appCategoryFilter} (${selectedInTarget})`)
          : (state.appCategoryFilter === "All" ? `Select all (${state.selected_apps.length}/${APP_STORE_CATALOG.length})` : `Select all in filter (${selectedInTarget}/${targetList.length})`);
      }
    }

    function refreshAppGridInPlace() {
      const grid = document.getElementById("appStoreGridContainer");
      if (grid) grid.innerHTML = renderFilteredAppCardsHtml();
      const pillsWrap = document.getElementById("appCategoryPills");
      if (pillsWrap) {
        pillsWrap.querySelectorAll(".filter-pill").forEach(btn => {
          btn.classList.toggle("active", btn.getAttribute("data-category") === state.appCategoryFilter);
        });
      }
      syncSelectAllCheckboxInPlace();
    }

    function attachSleekScrollListeners() {
      document.addEventListener("scroll", (e) => {
        const el = e.target;
        if (el && el.id === "flashLogBox") {
          if (el._programmaticScroll) return;
          const distFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
          const atBottom = distFromBottom <= 24;
          if (state.logAutoScroll !== atBottom) {
            state.logAutoScroll = atBottom;
            const resumeBtn = document.getElementById("btnResumeAutoScroll");
            if (resumeBtn) {
              resumeBtn.style.display = state.logAutoScroll ? "none" : "inline-flex";
            }
          }
        }
      }, true);


    }

    document.getElementById("btnThemeToggle").addEventListener("click", () => {
      state.theme = state.theme === "dark" ? "light" : "dark";
      document.documentElement.setAttribute("data-theme", state.theme);
      const themeLabel = document.getElementById("themeToggleLabel");
      if (themeLabel) {
        themeLabel.textContent = state.theme === "dark" ? "Light" : "Dark";
      }
    });

    document.getElementById("btnToggleModeView").addEventListener("click", () => {
      state.viewMode = state.viewMode === "setup" ? "settings" : "setup";
      state.openDropdownField = null;
      shouldAnimateTab = true;
      renderApp();
    });

    document.getElementById("btnTitleBack").addEventListener("click", () => {
      state.openDropdownField = null;
      if (state.viewMode === "setup") {
        const idx = SETUP_TABS.findIndex(t => t.id === state.setupTab);
        if (idx > 0) {
          state.setupTab = SETUP_TABS[idx - 1].id;
          shouldAnimateTab = true;
          renderApp();
        }
      } else {
        if (state.searchQuery) {
          state.searchQuery = "";
          document.getElementById("globalSearchInput").value = "";
          shouldAnimateTab = true;
          renderApp();
        } else if (state.settingsTab !== "system") {
          state.settingsTab = "system";
          shouldAnimateTab = true;
          renderApp();
        }
      }
    });

    document.getElementById("globalSearchInput").addEventListener("input", (e) => {
      state.searchQuery = e.target.value;
      renderSettingsView();
    });

    document.body.addEventListener("click", async (e) => {
      const target = e.target.closest("[data-action]");

      if (!e.target.closest(".win-combobox") && state.openDropdownField !== null) {
        state.openDropdownField = null;
        document.querySelectorAll(".win-combobox.open").forEach(cb => cb.classList.remove("open"));
        document.querySelectorAll(".expander-wrap.dropdown-open-inside").forEach(ex => ex.classList.remove("dropdown-open-inside"));
      }

      if (!target) return;
      const action = target.getAttribute("data-action");
      const allowedWhileFlashing = new Set([
        "pause-resume-flash",
        "cancel-flash",
        "resume-log-autoscroll",
        "dismiss-close-warning",
        "pause-and-exit-flash",
        "confirm-cancel-and-exit",
        "open-dashboard-preview",
        "copy-yaml"
      ]);
      if (state.flashing && action && !allowedWhileFlashing.has(action)) {
        showToast("Configuration is locked while USB flashing/downloading is active");
        return;
      }

      if (action === "toggle-dropdown") {
        const field = target.getAttribute("data-field");
        const box = target.closest(".win-combobox");
        const willOpen = state.openDropdownField !== field;
        document.querySelectorAll(".win-combobox.open").forEach(cb => cb.classList.remove("open", "drop-up", "align-right"));
        document.querySelectorAll(".win-card.has-open-dropdown").forEach(c => c.classList.remove("has-open-dropdown"));
        document.querySelectorAll(".expander-wrap.dropdown-open-inside").forEach(ex => ex.classList.remove("dropdown-open-inside"));
        state.openDropdownField = willOpen ? field : null;
        if (box && willOpen) {
          box.classList.add("open");
          const parentCard = box.closest(".win-card");
          if (parentCard) parentCard.classList.add("has-open-dropdown");
          const parentExp = box.closest(".expander-wrap.open");
          if (parentExp) parentExp.classList.add("dropdown-open-inside");
          const rect = box.getBoundingClientRect();
          if (window.innerHeight - rect.bottom < 250 && rect.top > 240) {
            box.classList.add("drop-up");
          }
          if (rect.left > window.innerWidth * 0.48) {
            box.classList.add("align-right");
          }
        }
        return;
      } else if (action === "select-dropdown-option") {
        const field = target.getAttribute("data-field");
        const val = target.getAttribute("data-value");
        const label = target.getAttribute("data-label") || val;
        if (field) {
          state[field] = val;
        }
        if (field === "laptop_age_hint") {
          if (val === "modern_uefi") state.usb_partition_scheme = "gpt_uefi";
          else if (val === "legacy_bios") state.usb_partition_scheme = "mbr_bios";
          else state.usb_partition_scheme = "auto";
          state.openDropdownField = null;
          renderApp();
          return;
        }
        if (field === "cluster_mode" || field === "cluster_storage_policy") {
          state.openDropdownField = null;
          renderApp();
          return;
        }
        if (field === "iso_source_mode") {
          const isoIn = document.getElementById("customIsoPathInput");
          if (isoIn) isoIn.style.display = val === "local_file" ? "" : "none";
        }
        state.openDropdownField = null;
        const box = target.closest(".win-combobox");
        if (box) {
          box.classList.remove("open");
          const trigLbl = box.querySelector(".win-combobox-label");
          if (trigLbl) trigLbl.textContent = label;
          box.querySelectorAll(".win-combobox-option").forEach(opt => {
            const sel = opt.getAttribute("data-value") === val;
            opt.classList.toggle("selected", sel);
            opt.setAttribute("aria-selected", String(sel));
          });
        }
        if (field === "screen_blank_seconds") {
          const blankTitle = document.getElementById("lblScreenBlankTitle");
          if (blankTitle) blankTitle.textContent = `Turn off screen after ${val} seconds`;
        }
        return;
      }

      if (action === "toggle-app-card") {
        const appId = target.getAttribute("data-app");
        if (!appId) return;
        const idx = state.selected_apps.indexOf(appId);
        if (idx >= 0) state.selected_apps.splice(idx, 1);
        else state.selected_apps.push(appId);
        const nowChecked = state.selected_apps.includes(appId);
        target.classList.toggle("checked", nowChecked);
        const cb = target.querySelector(".app-checkbox");
        if (cb) cb.checked = nowChecked;
        syncSelectAllCheckboxInPlace();
        return;
      } else if (action === "toggle-select-all-apps") {
        e.preventDefault();
        const filtered = getFilteredAppsList();
        const targetList = filtered.length > 0 ? filtered : APP_STORE_CATALOG;
        const allInFilterSelected = targetList.every(a => state.selected_apps.includes(a.id));
        if (allInFilterSelected) {
          const removeSet = new Set(targetList.map(a => a.id));
          state.selected_apps = state.selected_apps.filter(id => !removeSet.has(id));
        } else {
          const currentSet = new Set(state.selected_apps);
          targetList.forEach(a => currentSet.add(a.id));
          state.selected_apps = Array.from(currentSet);
        }
        refreshAppGridInPlace();
        return;
      } else if (action === "apps-select-all") {
        state.selected_apps = APP_STORE_CATALOG.map(a => a.id);
        refreshAppGridInPlace();
        return;
      } else if (action === "apps-select-recommended") {
        state.selected_apps = APP_STORE_CATALOG.filter(a => a.recommended).map(a => a.id);
        refreshAppGridInPlace();
        return;
      } else if (action === "apps-select-old-laptop") {
        state.selected_apps = ["jellyfin", "pihole", "kiwix", "vaultwarden", "filebrowser", "syncthing", "navidrome"];
        refreshAppGridInPlace();
        showToast("Selected lightweight pack for 2 GB – 4 GB old laptops");
        return;
      } else if (action === "apps-select-high-end") {
        state.selected_apps = ["jellyfin", "nextcloud", "pihole", "kiwix", "immich", "homeassistant", "vaultwarden", "minecraft", "ollama_webui"];
        refreshAppGridInPlace();
        showToast("Selected Higher-End Homelab + Minecraft + Local AI pack");
        return;
      } else if (action === "apps-unselect-all") {
        state.selected_apps = [];
        refreshAppGridInPlace();
        return;
      } else if (action === "filter-app-category") {
        state.appCategoryFilter = target.getAttribute("data-category") || "All";
        refreshAppGridInPlace();
        return;
      }

      if (action === "toggle-expander") {
        const key = target.getAttribute("data-expander");
        if (key) {
          state.advancedExpanded[key] = !state.advancedExpanded[key];
          const wrap = target.closest(".expander-wrap");
          if (wrap) {
            wrap.classList.remove("dropdown-open-inside");
            wrap.classList.toggle("open", state.advancedExpanded[key]);
          }
        }
        return;
      }

      if (action === "set-ui-mode") {
        const nextMode = target.getAttribute("data-mode") || "standard";
        state.ui_mode = nextMode;
        if (nextMode === "pro") {
          state.advancedExpanded = { network: true, apps: true, storage: true, power: true };
          showToast("Pro Mode enabled — Additional system, network & power options unlocked");
        } else {
          showToast("Standard Mode enabled — Guided setup with smart defaults");
        }
        renderApp();
        return;
      } else if (action === "elevate-admin") {
        await callBackend("save-config", "POST", serializeProfilePayload());
        const res = await callBackend("elevate", "POST", {});
        if (res && res.already_elevated) {
          state.hostIsAdmin = true;
          const adminBanner = document.getElementById("adminPrivilegeBanner");
          if (adminBanner) adminBanner.style.display = "none";
          showToast("Already running with Administrator privileges");
        } else if (res && res.elevated_spawned) {
          state.forceClosing = true;
          showToast("Administrator window opened — closing this window...");
          setTimeout(() => { window.close(); }, 180);
        } else {
          showToast((res && res.message) ? res.message : "Right-click InstantServer-Windows.exe and choose 'Run as administrator'");
        }
        return;
      } else if (action === "apply-role-preset") {
        const presetId = target.getAttribute("data-preset") || "";
        const preset = SERVER_ROLE_PRESETS.find(p => p.id === presetId);
        if (preset) {
          state.preset_role = preset.id;
          state.selected_apps = [...preset.apps];
          if (preset.vaapi !== undefined) state.enable_intel_vaapi = preset.vaapi;
          if (preset.samba !== undefined) state.enable_samba_nas = preset.samba;
          if (preset.pihole_dns !== undefined) state.enable_pihole_dns_prep = preset.pihole_dns;
          if (preset.zram !== undefined) state.enable_low_ram_zram = preset.zram;
          if (preset.governor) state.cpu_governor = preset.governor;
          if (preset.zims && preset.zims.length) state.preselect_kiwix_zim = preset.zims[0];
          showToast(`Applied '${preset.title}' preset (${preset.apps.length} apps selected)`);
          renderApp();
        }
        return;
      }

      if (action === "set-storage-unit") {
        const nextUnit = target.getAttribute("data-unit") || "GB";
        if (nextUnit !== state.storageUnit) {
          state.storageUnit = nextUnit;
          if (nextUnit === "TB") {
            const tbVal = +(state.dual_boot_Allocation_gb / 1024).toFixed(2);
            state.storageInputRaw = String(tbVal || 1);
          } else {
            state.storageInputRaw = String(state.dual_boot_Allocation_gb);
          }
          state.storageInputError = "";
          renderApp();
        }
        return;
      } else if (action === "set-storage-preset") {
        const gb = Number(target.getAttribute("data-gb")) || 250;
        state.dual_boot_Allocation_gb = gb;
        if (gb >= 1024 && gb % 1024 === 0) {
          state.storageUnit = "TB";
          state.storageInputRaw = String(gb / 1024);
        } else {
          state.storageUnit = "GB";
          state.storageInputRaw = String(gb);
        }
        state.storageInputError = "";
        renderApp();
        return;
      } else if (action === "toggle-wifi-powersave") {
        state.disable_wifi_power_save = !state.disable_wifi_power_save;
        updateSwitchInPlace(target, state.disable_wifi_power_save);
        return;
      } else if (action === "toggle-tethering") {
        state.enable_usb_tethering = !state.enable_usb_tethering;
        updateSwitchInPlace(target, state.enable_usb_tethering);
        return;
      } else if (action === "toggle-offline-images") {
        state.prebundle_offline_images = !state.prebundle_offline_images;
        updateSwitchInPlace(target, state.prebundle_offline_images);
        return;
      } else if (action === "toggle-pihole") {
        state.enable_pihole_dns_prep = !state.enable_pihole_dns_prep;
        updateSwitchInPlace(target, state.enable_pihole_dns_prep);
        return;
      } else if (action === "toggle-samba") {
        state.enable_samba_nas = !state.enable_samba_nas;
        updateSwitchInPlace(target, state.enable_samba_nas);
        return;
      } else if (action === "toggle-vaapi") {
        state.enable_intel_vaapi = !state.enable_intel_vaapi;
        updateSwitchInPlace(target, state.enable_intel_vaapi);
        return;
      } else if (action === "toggle-tailscale") {
        state.enable_tailscale = !state.enable_tailscale;
        document.querySelectorAll("[data-action='toggle-tailscale']").forEach(sw => updateSwitchInPlace(sw, state.enable_tailscale));
        const authIn = document.getElementById("tailscaleAuthInput");
        if (authIn) authIn.style.display = state.enable_tailscale ? "" : "none";
        const netRow = document.getElementById("tailscaleNetConfigRow");
        if (netRow) netRow.style.display = state.enable_tailscale ? "flex" : "none";
        return;
      } else if (action === "toggle-lid") {
        state.ignore_lid_switch = !state.ignore_lid_switch;
        updateSwitchInPlace(target, state.ignore_lid_switch);
        const thBanner = document.getElementById("bannerThermalOverheatTrap");
        if (thBanner) thBanner.style.display = state.ignore_lid_switch ? "" : "none";
        return;
      } else if (action === "enable-cool-quiet-profile") {
        const nextCool = !state.disable_cpu_turbo_boost;
        state.disable_cpu_turbo_boost = nextCool;
        state.cpu_governor = nextCool ? "powersave" : "schedutil";
        showToast(nextCool
          ? "Enabled Cool & Quiet CPU profile (Turbo Boost off + Quiet fan profile)"
          : "Disabled Cool & Quiet profile (Restored Balanced schedutil CPU speed)");
        renderApp();
        return;
      } else if (action === "toggle-zram") {
        state.enable_low_ram_zram = !state.enable_low_ram_zram;
        updateSwitchInPlace(target, state.enable_low_ram_zram);
        return;
      } else if (action === "toggle-nomodeset") {
        state.safe_gpu_nomodeset = !state.safe_gpu_nomodeset;
        updateSwitchInPlace(target, state.safe_gpu_nomodeset);
        return;
      } else if (action === "toggle-auto-updates") {
        state.enable_auto_updates = !state.enable_auto_updates;
        updateSwitchInPlace(target, state.enable_auto_updates);
        return;
      } else if (action === "toggle-scheduled-backups") {
        state.enable_scheduled_backups = !state.enable_scheduled_backups;
        updateSwitchInPlace(target, state.enable_scheduled_backups);
        return;
      } else if (action === "toggle-prometheus") {
        state.enable_prometheus_metrics = !state.enable_prometheus_metrics;
        updateSwitchInPlace(target, state.enable_prometheus_metrics);
        return;
      } else if (action === "toggle-tcp-bbr") {
        state.enable_tcp_bbr = !state.enable_tcp_bbr;
        updateSwitchInPlace(target, state.enable_tcp_bbr);
        return;
      } else if (action === "toggle-fail2ban") {
        state.enable_fail2ban = !state.enable_fail2ban;
        updateSwitchInPlace(target, state.enable_fail2ban);
        return;
      } else if (action === "toggle-disable-turbo") {
        state.disable_cpu_turbo_boost = !state.disable_cpu_turbo_boost;
        updateSwitchInPlace(target, state.disable_cpu_turbo_boost);
        return;
      } else if (action === "toggle-confirm-erase") {
        state.confirm_erase_main_drive = !!target.checked;
        const banner = document.getElementById("bannerAutoErase");
        if (banner) banner.classList.toggle("notice-banner-warning", state.confirm_erase_main_drive);
        return;
      } else if (action === "toggle-confirm-wipe-secondary") {
        state.confirm_wipe_secondary = !!target.checked;
        return;
      } else if (action === "toggle-confirm-usb-erase") {
        state.confirm_usb_erase = !!target.checked;
        return;
      } else if (action === "toggle-confirm-wipe-server-data") {
        state.confirm_wipe_server_data = !!target.checked;
        return;
      } else if (action === "toggle-iphone-backup") {
        state.enable_iphone_auto_backup = !state.enable_iphone_auto_backup;
        if (state.enable_iphone_auto_backup && !state.selected_apps.includes("immich")) {
          state.selected_apps.push("immich");
        }
        renderApp();
        return;
      } else if (action === "load-workflow-template") {
        const tpl = target.getAttribute("data-tpl") || "samba_share";
        if (tpl === "samba_share") {
          state.custom_config_path = "/etc/samba/smb-custom.conf";
          state.custom_config_body = "[CustomShare]\n   comment = Custom Network Share\n   path = /DATA/CustomShare\n   browseable = yes\n   read only = no\n   guest ok = no\n   vfs objects = catia fruit streams_xattr\n";
        } else if (tpl === "usb_udev") {
          state.custom_config_path = "/etc/udev/rules.d/99-custom-usb-trigger.rules";
          state.custom_config_body = "# Automatically run a custom script whenever a specific USB device is plugged in\nACTION==\"add\", SUBSYSTEM==\"usb\", RUN+=\"/DATA/CustomWorkflows/on-usb-plug.sh\"\n";
        } else if (tpl === "systemd_timer") {
          state.custom_config_path = "/etc/systemd/system/custom-automation.service";
          state.custom_config_body = "[Unit]\nDescription=Custom Server Background Automation\nAfter=network-online.target\n\n[Service]\nType=oneshot\nExecStart=/bin/bash /DATA/CustomWorkflows/custom-job.sh\n";
        }
        showToast("Loaded custom workflow template — edit as needed and click + Stage Custom File");
        renderApp();
        return;
      } else if (action === "save-custom-sys-config") {
        const p = (state.custom_config_path || "").trim();
        if (!p.startsWith("/")) {
          showToast("Enter an absolute target path starting with / (e.g. /etc/samba/smb-custom.conf)");
          return;
        }
        state.custom_config_files[p] = state.custom_config_body || "";
        showToast(`Staged custom config file ${p} for USB provisioning`);
        renderApp();
        return;
      } else if (action === "toggle-mc-online-mode") {
        state.mc_online_mode = !!target.checked;
        renderApp();
        return;
      } else if (action === "select-studio-file") {
        const fn = target.getAttribute("data-file");
        if (fn) {
          state.studio_file_name = fn;
          renderApp();
        }
        return;
      } else if (action === "add-studio-file") {
        const rawFn = (state.studio_new_file_name || "").trim().replace(/[^a-zA-Z0-9._-]/g, "");
        if (!rawFn) {
          showToast("Enter a file name first (e.g. custom.conf or whitelist.json)");
          return;
        }
        const ov = getOrInitAppOverride(state.studio_app_id || "minecraft");
        if (!ov.config_files[rawFn]) {
          ov.config_files[rawFn] = `# /DATA/AppData/${state.studio_app_id || "minecraft"}/${rawFn}\n`;
        }
        state.studio_file_name = rawFn;
        state.studio_new_file_name = "";
        showToast(`Created ${rawFn} in /DATA/AppData/${state.studio_app_id || "minecraft"}`);
        renderApp();
        return;
      } else if (action === "toggle-mc-aikar") {
        state.mc_use_aikar_flags = !!target.checked;
        state.mc_enable_backups = !!target.checked;
        return;
      }

      if (action === "switch-setup-tab" || action === "jump-setup-tab") {
        const nextTab = target.getAttribute("data-tab") || "network";
        if (state.viewMode !== "setup") state.viewMode = "setup";
        if (state.setupTab !== nextTab) shouldAnimateTab = true;
        state.setupTab = nextTab;
        state.validationErrors = [];
        state.openDropdownField = null;
        renderApp();
      } else if (action === "setup-prev") {
        const idx = SETUP_TABS.findIndex(t => t.id === state.setupTab);
        if (idx > 0) {
          state.setupTab = SETUP_TABS[idx - 1].id;
          state.openDropdownField = null;
          shouldAnimateTab = true;
          renderApp();
        }
      } else if (action === "setup-skip" || action === "setup-next") {
        const idx = SETUP_TABS.findIndex(t => t.id === state.setupTab);
        if (idx < SETUP_TABS.length - 1) {
          state.setupTab = SETUP_TABS[idx + 1].id;
          state.openDropdownField = null;
          shouldAnimateTab = true;
          renderApp();
        }
      } else if (action === "switch-settings-tab") {
        const nextTab = target.getAttribute("data-tab") || "system";
        if (state.settingsTab !== nextTab || state.searchQuery) shouldAnimateTab = true;
        state.settingsTab = nextTab;
        state.searchQuery = "";
        state.openDropdownField = null;
        const sInput = document.getElementById("globalSearchInput");
        if (sInput) sInput.value = "";
        renderApp();
      } else if (action === "settings-prev") {
        const idx = SETTINGS_TABS.findIndex(t => t.id === state.settingsTab);
        if (idx > 0) {
          state.settingsTab = SETTINGS_TABS[idx - 1].id;
          state.openDropdownField = null;
          shouldAnimateTab = true;
          renderApp();
        }
      } else if (action === "settings-next") {
        const idx = SETTINGS_TABS.findIndex(t => t.id === state.settingsTab);
        if (idx < SETTINGS_TABS.length - 1) {
          state.settingsTab = SETTINGS_TABS[idx + 1].id;
          state.openDropdownField = null;
          shouldAnimateTab = true;
          renderApp();
        }
      } else if (action === "clear-search") {
        state.searchQuery = "";
        document.getElementById("globalSearchInput").value = "";
        shouldAnimateTab = true;
        renderApp();
      } else if (action === "select-dashboard") {
        state.dashboard_choice = target.getAttribute("data-value") || "both";
        renderApp();
      } else if (action === "select-install-mode") {
        state.install_mode = target.getAttribute("data-value") || "dual_boot";
        if (state.install_mode !== "auto_erase") {
          state.confirm_erase_main_drive = false;
        }
        renderApp();
      } else if (action === "select-usb") {
        state.selectedUsbId = target.getAttribute("data-usb") || "";
        renderApp();
      } else if (action === "scan-usb") {
        await scanUsbDrives(false);
      } else if (action === "reset-usb-drive") {
        await resetSelectedUsbDrive();
      } else if (action === "save-config-home") {
        await saveConfigurationAndOpenHome();
      } else if (action === "start-flash") {
        await startFlashingProcess();
      } else if (action === "pause-resume-flash") {
        await togglePauseResumeFlashing();
      } else if (action === "add-custom-web-app") {
        await addCustomWebAppFromInput();
      } else if (action === "cancel-flash") {
        await cancelFlashingProcess(false);
      } else if (action === "resume-log-autoscroll") {
        state.logAutoScroll = true;
        scrollTerminalToBottomIfSticky();
      } else if (action === "dismiss-close-warning") {
        state.closeWarningOpen = false;
        const modal = document.getElementById("closeWarningModal");
        if (modal) modal.classList.remove("open");
      } else if (action === "pause-and-exit-flash") {
        const modal = document.getElementById("closeWarningModal");
        if (modal) modal.classList.remove("open");
        await pauseAndExitFlashingProcess();
      } else if (action === "confirm-cancel-and-exit") {
        const modal = document.getElementById("closeWarningModal");
        if (modal) modal.classList.remove("open");
        await cancelFlashingProcess(true);
      } else if (action === "cache-keep") {
        await handleCacheRetentionAction("keep");
      } else if (action === "cache-delete") {
        await handleCacheRetentionAction("delete");
      } else if (action === "toggle-expand-apps") {
        state.appsExpanded = !state.appsExpanded;
        refreshAppGridInPlace();
      } else if (action === "open-dashboard-preview") {
        window.open("server_dashboard.html", "_blank", "noopener");
        showToast("Opened live Server Web Dashboard preview");
      } else if (action === "copy-yaml") {
        const text = generateYamlPreview();
        try {
          await navigator.clipboard.writeText(text);
        } catch (_e) {
          const ta = document.createElement("textarea");
          ta.value = text;
          document.body.appendChild(ta);
          ta.select();
          document.execCommand("copy");
          document.body.removeChild(ta);
        }
        showToast("Copied YAML configuration");
      } else if (action === "scan-lan") {
        state.lanScanning = true;
        renderApp();
        const res = await callBackend("discover", "GET");
        state.lanScanning = false;
        if (res && Array.isArray(res.servers_found) && res.servers_found.length > 0) {
          state.lanScanResults = res.servers_found;
          showToast(`Found ${res.servers_found.length} server(s)`);
        } else {
          state.lanScanResults = [
            { ip: `${state.hostname || "instant-server"}.local`, open_ports: [22, 9090, 80] }
          ];
          showToast("LAN scan complete");
        }
        renderApp();
      }
    });

    document.body.addEventListener("change", (e) => {
      if (state.flashing) return;
      const act = e.target.getAttribute("data-action");
      if (act === "select-studio-app") {
        state.studio_app_id = e.target.value;
        const ov = getOrInitAppOverride(state.studio_app_id);
        state.studio_file_name = Object.keys(ov.config_files)[0] || ".env";
        renderApp();
      }
    });

    document.body.addEventListener("input", (e) => {
      if (state.flashing) return;
      const act = e.target.getAttribute("data-action");
      if (act === "studio-file-textarea") {
        const ov = getOrInitAppOverride(state.studio_app_id || "minecraft");
        if (state.studio_file_name) {
          ov.config_files[state.studio_file_name] = e.target.value;
        }
        return;
      } else if (act === "studio-port-input") {
        const ov = getOrInitAppOverride(state.studio_app_id || "minecraft");
        ov.host_port = Number(e.target.value) || 8080;
        return;
      } else if (act === "studio-mem-input") {
        const ov = getOrInitAppOverride(state.studio_app_id || "minecraft");
        ov.memory_limit = e.target.value.trim();
        return;
      }
      const field = e.target.getAttribute("data-field");
      if (!field) return;
      state[field] = e.target.value;

      if (field === "appSearchQuery") {
        refreshAppGridInPlace();
      } else if (field === "hostname") {
        const hostDesc = document.getElementById("descServerHostname");
        if (hostDesc) {
          hostDesc.textContent = `Name used to reach your server on your home network (${state.hostname || "instant-server"}.local)`;
        }
      } else if (field === "storage_custom_input") {
        const rawStr = String(e.target.value || "").trim();
        state.storageInputRaw = rawStr;
        const num = Number(rawStr);
        const computedGb = state.storageUnit === "TB" ? Math.round(num * 1024) : Math.round(num);
        const lbl = document.getElementById("lblDualBootSize");
        const valMsg = document.getElementById("lblStorageValidation");
        if (!rawStr || !Number.isFinite(num) || num <= 0) {
          state.storageInputError = "Enter a valid positive number (15 GB – 64 TB).";
          e.target.classList.add("invalid");
        } else if (computedGb < 15) {
          state.storageInputError = "Minimum partition size is 15 GB (for Ubuntu Server + Docker).";
          e.target.classList.add("invalid");
        } else if (computedGb > 65536) {
          state.storageInputError = "Maximum partition size is 64 TB (65,536 GB).";
          e.target.classList.add("invalid");
        } else {
          state.storageInputError = "";
          state.dual_boot_Allocation_gb = computedGb;
          e.target.classList.remove("invalid");
        }
        const disp = state.dual_boot_Allocation_gb >= 1024 && (state.dual_boot_Allocation_gb % 1024 === 0)
          ? `${state.dual_boot_Allocation_gb / 1024} TB (${state.dual_boot_Allocation_gb} GB)`
          : `${state.dual_boot_Allocation_gb} GB`;
        if (lbl) lbl.textContent = `Server partition size (${disp})`;
        if (valMsg) {
          valMsg.textContent = state.storageInputError || "Type any size from 15 GB up to 64 TB (65,536 GB) or pick a preset below";
          valMsg.style.color = state.storageInputError ? "var(--danger)" : "";
          valMsg.style.fontWeight = state.storageInputError ? "600" : "";
        }
      } else if (field === "battery_charge_stop_pct") {
        const t = document.getElementById("lblBatCapTitle");
        const d = document.getElementById("lblBatCapDesc");
        if (t) t.textContent = `Battery health limiter (${state.battery_charge_start_pct}% – ${state.battery_charge_stop_pct}%)`;
        if (d) d.textContent = `Caps charging at ${state.battery_charge_stop_pct}% while plugged in 24/7 to protect the battery`;
      } else if (field === "ups_shutdown_battery_pct") {
        const t = document.getElementById("lblUpsTitle");
        const d = document.getElementById("lblUpsDesc");
        if (t) t.textContent = `Safe shutdown during power outage (${state.ups_shutdown_battery_pct}%)`;
        if (d) d.textContent = `Uses the laptop battery as a built-in UPS and shuts down cleanly when battery drops to ${state.ups_shutdown_battery_pct}%`;
      } else if (field === "vm_swappiness") {
        const t = document.getElementById("lblSwappinessTitle");
        if (t) t.textContent = `Memory swap tendency (${state.vm_swappiness}%)`;
      }

      if (e.target && e.target.type === "range") {
        updateSliderBubblePosition(e.target, true);
      }
    });

    function updateSliderBubblePosition(rangeEl, showBubble) {
      if (!rangeEl) return;
      const wrap = rangeEl.closest(".win-slider-wrap");
      if (!wrap) return;
      const bubble = wrap.querySelector(".win-slider-bubble");
      if (!bubble) return;
      const min = Number(rangeEl.min) || 0;
      const max = Number(rangeEl.max) || 100;
      const val = Number(rangeEl.value ?? min);
      const unit = rangeEl.getAttribute("data-unit") || "%";
      const ratio = Math.max(0, Math.min(1, (val - min) / Math.max(1, max - min)));
      const trackWidth = rangeEl.offsetWidth || 160;
      const leftPx = 8 + ratio * Math.max(0, trackWidth - 16);
      bubble.style.left = `${leftPx.toFixed(1)}px`;
      bubble.textContent = `${val}${unit}`;
      if (showBubble) {
        wrap.classList.add("active");
        if (wrap._hideTimer) clearTimeout(wrap._hideTimer);
      }
    }

    function hideSliderBubble(rangeEl) {
      if (!rangeEl) return;
      const wrap = rangeEl.closest(".win-slider-wrap");
      if (!wrap) return;
      if (wrap._hideTimer) clearTimeout(wrap._hideTimer);
      wrap.classList.remove("active");
    }

    document.body.addEventListener("pointerdown", (e) => {
      if (e.target && e.target.type === "range") {
        updateSliderBubblePosition(e.target, true);
      }
    });
    document.body.addEventListener("pointerup", (e) => {
      if (e.target && e.target.type === "range") {
        hideSliderBubble(e.target);
      }
    });
    document.body.addEventListener("pointercancel", (e) => {
      if (e.target && e.target.type === "range") {
        hideSliderBubble(e.target);
      }
    });
    document.body.addEventListener("mouseout", (e) => {
      if (e.target && e.target.type === "range" && !e.target.matches(":active")) {
        hideSliderBubble(e.target);
      }
    });
    document.body.addEventListener("blur", (e) => {
      if (e.target && e.target.type === "range") {
        hideSliderBubble(e.target);
      }
    }, true);

    window.__showCloseWhileFlashingWarning = () => {
      state.closeWarningOpen = true;
      const modal = document.getElementById("closeWarningModal");
      if (modal) modal.classList.add("open");
    };

    window.addEventListener("pagehide", () => {
      try {
        if (window.location.protocol.startsWith("http") && navigator.sendBeacon) {
          const blob = new Blob([JSON.stringify({ force: true })], { type: "application/json" });
          navigator.sendBeacon(window.location.origin + "/api/shutdown", blob);
        }
      } catch (_err) {}
    });

    function attachWailsEventChannelListeners() {
      if (!window.runtime || typeof window.runtime.EventsOn !== "function") return;
      let rafPending = false;
      window.runtime.EventsOn("flash:progress", (ev) => {
        if (!ev) return;
        state.lastWailsEventTs = Date.now();
        state.flashing = true;
        const nextOverall = Number(ev.percent ?? ev.pct ?? state.flashProgress) || state.flashProgress;
        const nextStagePct = Number(ev.stage_percent ?? nextOverall) || nextOverall;
        state.flashProgress = Math.max(state.flashProgress, nextOverall);
        if (ev.eta_sec !== undefined && ev.eta_sec > 0) {
          const remMin = String(Math.floor(ev.eta_sec / 60)).padStart(2, "0");
          const remSec = String(Math.floor(ev.eta_sec % 60)).padStart(2, "0");
          state.flashEtaText = `ETA: ~${remMin}:${remSec} remaining`;
        }
        state.flashStatusText = ev.message || "Streaming aligned DMA blocks...";
        state.activeProgressLine = buildAsciiTerminalBar(nextStagePct, ev.stage || "Active Stage", ev.message || "");
        if (!rafPending) {
          rafPending = true;
          window.requestAnimationFrame(() => {
            rafPending = false;
            updateFlashPanelInPlace();
          });
        }
      });
      window.runtime.EventsOn("flash:paused", () => {
        state.flashPaused = true;
        updateFlashPanelInPlace();
      });
      window.runtime.EventsOn("flash:resumed", () => {
        state.flashPaused = false;
        updateFlashPanelInPlace();
      });
      window.runtime.EventsOn("flash:complete", (ev) => {
        state.flashing = false;
        state.flashPaused = false;
        state.flashProgress = 100;
        state.activeProgressLine = "";
        state.flashEtaText = "";
        state.flashStatusText = "Bootable USB drive is ready! Plug it into your computer and power it on.";
        if (ev && ev.sha256) {
          state.flashLogs.push(`[VERIFIED] SHA-256: ${ev.sha256} • CRC-32: ${ev.crc32 || "OK"}`);
        }
        updateFlashPanelInPlace();
        showToast("Bootable Instant Server USB is ready");
      });
      window.runtime.EventsOn("flash:error", (ev) => {
        state.flashing = false;
        state.flashPaused = false;
        state.activeProgressLine = "";
        const errMsg = (ev && ev.error) ? ev.error : "USB write interrupted";
        state.flashStatusText = `Failed: ${errMsg}`;
        state.flashLogs.push(`[ERROR] ${errMsg}`);
        updateFlashPanelInPlace();
      });
      window.runtime.EventsOn("flash:close-warning", () => {
        if (window.__showCloseWhileFlashingWarning) {
          window.__showCloseWhileFlashingWarning();
        }
      });
    }

    window.addEventListener("load", () => {
      attachSleekScrollListeners();
      attachWailsEventChannelListeners();
      initAppStartup();
      setInterval(async () => {
        const hb = await callBackend("heartbeat", "POST", {});
        if (hb && hb.close_warning_requested && window.__showCloseWhileFlashingWarning) {
          window.__showCloseWhileFlashingWarning();
        }
      }, 1000);
    });