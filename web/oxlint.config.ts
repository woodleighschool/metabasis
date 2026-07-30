export default {
  ignorePatterns: ["dist/**"],

  env: {
    builtin: true,
    browser: true,
  },

  options: {
    typeAware: true,
    reportUnusedDisableDirectives: "error",
    denyWarnings: true,
  },

  plugins: ["eslint", "typescript", "unicorn", "oxc", "import", "react", "jsx-a11y", "promise"],

  categories: {
    correctness: "error",
    suspicious: "error",
  },

  rules: {
    "react/react-in-jsx-scope": "off",
    "import/no-unassigned-import": ["error", { allow: ["**/*.css", "dayjs/locale/**"] }],
  },

  overrides: [
    {
      files: ["vite.config.ts"],
      env: {
        node: true,
      },
    },
  ],
};
