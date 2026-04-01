return {
  {
    "AstroNvim/astrocore",
    ---@type AstroCoreOpts
    opts = {
      mappings = {
        -- first key is the mode
        n = {
          -- ["C-S-Up"] = { ":m -2<CR>", desc = "Move line up" },
          -- ["C-S-Down"] = { ":m +1<CR>", desc = "Move line down" },
        },
        v = {
          -- ["<M-k>"] = { ":m '<-2<CR>gv=gv", desc = "move line up" },
          -- ["<M-j>"] = { ":m '>+1<CR>gv=gv", desc = "move line down" },
        },
      },
    },
  },
}
