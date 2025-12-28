-- Simple buffer selection mode for Kitty

-- Function to copy selection via xclip and quit
local function copy_and_quit()
  -- Yank to default register
  vim.cmd('normal! y')
  -- Get yanked text
  local text = vim.fn.getreg('"')
  -- Send to xclip
  vim.fn.system('xclip -selection clipboard', text)
  -- Quit
  vim.cmd('quit!')
end

-- Visual mode: y copies selection and quits
vim.keymap.set('v', 'y', copy_and_quit)

-- Normal mode: y copies current line and quits
vim.keymap.set('n', 'y', function()
  vim.cmd('normal! V')
  copy_and_quit()
end)

-- Quit bindings
vim.keymap.set({'n', 'v'}, 'q', function() vim.cmd('quit!') end)
vim.keymap.set({'n', 'v'}, '<Esc>', function() vim.cmd('quit!') end)

-- Quick select-all and copy: 'a' in normal mode
vim.keymap.set('n', 'a', function()
  vim.cmd('normal! ggVG')
  copy_and_quit()
end)

-- Disable line numbers
vim.opt.number = false
vim.opt.relativenumber = false

-- Go to bottom
vim.cmd('normal! G')

-- Show help
print('j/k move, V select, / search, y copy+quit, a all+copy, q/Esc quit')
