import js from "@eslint/js";
import eslintPluginPrettierRecommended from "eslint-plugin-prettier/recommended";
import react from "eslint-plugin-react";
import globals from "globals";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import tseslint from "typescript-eslint";
import importPlugin from "eslint-plugin-import";
import unusedImports from "eslint-plugin-unused-imports";

export default tseslint.config(
  { ignores: ["dist", "node_modules"] },
  {
    extends: [
      js.configs.recommended,
      ...tseslint.configs.recommended,
      eslintPluginPrettierRecommended,
      importPlugin.flatConfigs.recommended,
      importPlugin.flatConfigs.typescript,
    ],

    files: ["**/*.{ts,tsx}"],

    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },

    settings: {
      react: {
        version: "detect",
      },
      "import/resolver": {
        typescript: true,
        node: true,
      },
    },

    plugins: {
      importPlugin,
      react,
      "unused-imports": unusedImports,
      "react-hooks": reactHooks,
      "react-refresh": reactRefresh,
    },

    rules: {
      ...reactHooks.configs.recommended.rules,

      "react-refresh/only-export-components": [
        "warn",
        { allowConstantExport: true },
      ],

      "unused-imports/no-unused-imports": "error",
      "unused-imports/no-unused-vars": [
        "warn",
        {
          vars: "all",
          varsIgnorePattern: "^_",
          args: "after-used",
          argsIgnorePattern: "^_",
        },
      ],

      "padding-line-between-statements": [
        "warn",
        {
          blankLine: "always",
          prev: "*",
          next: "return",
        },
        {
          blankLine: "always",
          prev: ["const", "let", "var"],
          next: "*",
        },
        {
          blankLine: "any",
          prev: ["const", "let", "var"],
          next: ["const", "let", "var"],
        },
      ],

      "react/self-closing-comp": "warn",
      "react/jsx-sort-props": [
        "warn",
        {
          callbacksLast: true,
          shorthandFirst: true,
          noSortAlphabetically: false,
          reservedFirst: true,
        },
      ],

      "import/order": [
        "warn",
        {
          groups: [
            "type",
            "builtin",
            "object",
            "external",
            "internal",
            "parent",
            "sibling",
            "index",
          ],
          "newlines-between": "always",
          pathGroups: [
            {
              pattern: "~/**",
              group: "external",
              position: "after",
            },
          ],
        },
      ],
    },
  },
);
