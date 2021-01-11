# bing-wallpaper

## Description

A Ubuntu utility to download the daily Bing wallpaper and install it as the desktop background and lock screen. You can have the option to choose one of this weeks images.



Command line options (Go style)

*  -**clean**: Keep max. 10 recent images and remove the older others (default true).

* -**imgDir**: Image directory (default "~/Pictures/BingWallpaper"). The location where the image will be saved.
* -**imgOpt**: none, wallpaper, centered, scaled, stretched, zoom, spanned (default "zoom"). How the image will be rendered to the screen if the resolution does not match exactly.
* -**index**: 0=today, 1=yesterday, ... 7. Only the last 8 images are available on Bing.
* -**info**: Image meta info, no download, no clean. This is handy to check which images are available without downloading them.
* -**market**: en-US, zh-CN, ja-JP, en-AU, en-UK, de-DE, en-NZ, en-CA (default "en-UK"). Different markets have different images and the language of the description can be different.
* -**res**: Preferred resolution 1024x768, 1280x720, 1366x768, 1920x1080, 1920x1200 (default "1920x1080"). 