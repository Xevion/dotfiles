-- AstroCommunity: import any community modules here
-- We import this file in `lazy_setup.lua` before the `plugins/` folder.
-- This guarantees that the specs are processed before any user plugins.

---@type LazySpec
return {
  "AstroNvim/astrocommunity",
  { import = "astrocommunity.pack.lua" },
  { import = "astrocommunity.pack.rust" },
  { import = "astrocommunity.pack.tailwindcss" },
  { import = "astrocommunity.pack.typescript" },
  { import = "astrocommunity.pack.docker" },
  { import = "astrocommunity.pack.java" },
  { import = "astrocommunity.pack.json" },
  { import = "astrocommunity.pack.lua" },
  { import = "astrocommunity.pack.python" },
  { import = "astrocommunity.pack.html-css" },
  { import = "astrocommunity.pack.go" },
  { import = "astrocommunity.pack.prisma" },
  { import = "astrocommunity.pack.kotlin" },
  { import = "astrocommunity.pack.proto" },
  { import = "astrocommunity.pack.cpp" },
  {
    import = "astrocommunity.media.presence-nvim",
    opts = {
      client_id = "793271441293967371",
      main_image = "file",
    },
  },
  { import = "astrocommunity.git.git-blame-nvim" },
  { import = "astrocommunity.colorscheme.catppuccin" },
  { import = "astrocommunity.scrolling.nvim-scrollbar" },
  { import = "astrocommunity.scrolling.neoscroll-nvim" },
  { import = "astrocommunity.motion.nvim-surround" },
  { import = "astrocommunity.motion.mini-move" },
  { import = "astrocommunity.media.vim-wakatime" },
  { import = "astrocommunity.diagnostics.trouble-nvim" },
  {
    import = "astrocommunity.completion.copilot-lua-cmp",
    opts = {
      suggestion = {
        keymap = {},
      },
    },
  },
  {
    import = "astrocommunity.editing-support.todo-comments-nvim",
    opts = {
      colors = {
        info = { "#0D0D0F" },
      },
    },
  },
}
