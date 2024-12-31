mkdir dmg && cp -r EasyLPAC.app dmg/
mkdir dmg/Sources && cp -r lpac-*.zip LICENSE* dmg/Sources
ln -s /Applications dmg/Applications
hdiutil create -volname "EasyLPAC" -srcfolder dmg -ov -format UDRW EasyLPAC.dmg
hdiutil attach EasyLPAC.dmg
cp ../assets/icon.icns /Volumes/EasyLPAC/.VolumeIcon.icns
SetFile -c icnC /Volumes/EasyLPAC/.VolumeIcon.icns
SetFile -a C /Volumes/EasyLPAC

osascript <<EOD
tell application "Finder"
  tell disk "EasyLPAC"
    open
    set current view of container window to icon view
    set toolbar visible of container window to false
    set statusbar visible of container window to false
    set the bounds of container window to {400, 100, 1060, 540}
    set viewOptions to the icon view options of container window
    set arrangement of viewOptions to not arranged
    set icon size of viewOptions to 72
    set position of item "EasyLPAC.app" of container window to {230, 130}
    set position of item "Applications" of container window to {430, 130}
    set position of item "Sources" of container window to {330, 270}
    update without registering applications
    delay 5
    end tell
  end tell
EOD

hdiutil detach -force /Volumes/EasyLPAC
hdiutil convert EasyLPAC.dmg -format UDZO -o EasyLPAC-macOS-arm64-with-lpac.dmg