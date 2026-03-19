// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) OwnPulse Contributors

/** @type {import('tailwindcss').Config} */
export default {
  content: ["./src/**/*.{astro,html,js,jsx,md,mdx,svelte,ts,tsx,vue}"],
  theme: {
    extend: {
      colors: {
        primary: {
          DEFAULT: "#c2654a",
          light: "#d4856e",
          dark: "#9e4f38",
        },
        accent: {
          DEFAULT: "#3d8b8b",
          light: "#5aadad",
          dark: "#2d6b6b",
        },
        neutral: {
          50: "#f7f7f4",
          100: "#eeeeea",
          200: "#deded6",
          300: "#c2c2b9",
          400: "#9c9c93",
          500: "#7a7a72",
          600: "#5e5e57",
          700: "#44443f",
          800: "#2d2d2a",
          900: "#1e1e1c",
        },
        surface: {
          DEFAULT: "#fafaf7",
          elevated: "#ffffff",
        },
        success: "#5a8a5a",
        warning: "#c49a3c",
        error: "#b54a4a",
      },
      fontFamily: {
        display: [
          "Source Serif 4",
          "Source Serif Pro",
          "Georgia",
          "Times New Roman",
          "serif",
        ],
        body: [
          "IBM Plex Sans",
          "Helvetica Neue",
          "Arial",
          "sans-serif",
        ],
        mono: ["IBM Plex Mono", "SF Mono", "Fira Code", "monospace"],
      },
      fontSize: {
        xs: "0.75rem",
        sm: "0.875rem",
        base: "1rem",
        lg: "1.125rem",
        xl: "1.25rem",
        "2xl": "1.5rem",
        "3xl": "2rem",
        "4xl": "2.75rem",
        "5xl": "3.5rem",
      },
    },
  },
  plugins: [],
};
