/** @type {import('tailwindcss').Config} */
export default {
  content: ["./css/**/*.css", "./templates/**/*"],
  theme: {
    extend: {},
  },
  plugins: [require("@tailwindcss/typography")],
};
