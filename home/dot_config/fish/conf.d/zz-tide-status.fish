# Tide's stock status item renders nothing for a plain exit 1 when `character`
# is in the left prompt, leaving only a red prompt glyph. Defining the function
# here pre-empts autoload, so the numeric code is always shown.
function _tide_item_status
    if string match -qv 0 $_tide_pipestatus
        fish_status_to_signal $_tide_pipestatus | string replace SIG '' | string join '|' | read -l out
        if test $_tide_status = 0
            _tide_print_item status $tide_status_icon' ' $out
        else
            tide_status_bg_color=$tide_status_bg_color_failure \
                tide_status_color=$tide_status_color_failure \
                _tide_print_item status $tide_status_icon_failure' ' $out
        end
    else if not contains character $_tide_left_items
        _tide_print_item status $tide_status_icon
    end
end
