#!/bin/bash

APP_NAME="MonarchUpload"
EXECUTABLE_NAME="MonarchUpload"
IDENTIFIER="dev.eintim.monarchupload"
ICON_PATH="./AppIcon.icns"

echo "Building x86_64 binary..."
GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 CGO_CFLAGS="-mmacosx-version-min=11.00" CGO_LDFLAGS="-mmacosx-version-min=11.00" go build -o $EXECUTABLE_NAME-x86_64

# Build arm64 binary
echo "Building arm64 binary..."
GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 CGO_CFLAGS="-mmacosx-version-min=11.00" CGO_LDFLAGS="-mmacosx-version-min=11.00" go build -o $EXECUTABLE_NAME-arm64

# Combine binaries using lipo
echo "Creating universal binary..."
lipo -create -output $EXECUTABLE_NAME $EXECUTABLE_NAME-x86_64 $EXECUTABLE_NAME-arm64
rm $EXECUTABLE_NAME-x86_64
rm $EXECUTABLE_NAME-arm64

# Verify the universal binary
echo "Verifying universal binary..."
lipo -info $EXECUTABLE_NAME

# Create the .app bundle structure
mkdir -p $APP_NAME.app/Contents/MacOS
mkdir -p $APP_NAME.app/Contents/Resources

# Move the executable
mv $EXECUTABLE_NAME $APP_NAME.app/Contents/MacOS/

# Create the Info.plist file
cat <<EOL > $APP_NAME.app/Contents/Info.plist
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>$EXECUTABLE_NAME</string>
    <key>CFBundleIdentifier</key>
    <string>$IDENTIFIER</string>
    <key>CFBundleName</key>
    <string>$APP_NAME</string>
    <key>CFBundleVersion</key>
    <string>1.0</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleIconFile</key>
    <string>AppIcon</string>
</dict>
</plist>
EOL

# Add the icon if it exists
if [ -f "$ICON_PATH" ]; then
    cp "$ICON_PATH" $APP_NAME.app/Contents/Resources/
fi
cp -rf ./Resources/* $APP_NAME.app/Contents/Resources/
# Set executable permissions
chmod +x $APP_NAME.app/Contents/MacOS/$EXECUTABLE_NAME

echo "$APP_NAME.app bundle created successfully."
codesign -f -s - $APP_NAME.app/Contents/MacOS/$EXECUTABLE_NAME