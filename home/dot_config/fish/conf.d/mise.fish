# Activate mise automatically on shell startup
if command -q mise
    mise activate fish | source
end
