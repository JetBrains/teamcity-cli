#!/bin/bash
# Initialize TeamCity with internal HSQLDB database for non-interactive setup

CONFIG_DIR="/data/teamcity_server/datadir/config"

# Create config directory if it doesn't exist
mkdir -p "$CONFIG_DIR"

# Create database.properties for internal HSQLDB database if it doesn't exist
if [ ! -f "$CONFIG_DIR/database.properties" ]; then
    echo "Creating database.properties for internal HSQLDB..."
    cat > "$CONFIG_DIR/database.properties" << 'EOF'
# TeamCity internal HSQLDB database configuration
connectionUrl=jdbc:hsqldb:file:$TEAMCITY_SYSTEM_PATH/buildserver
EOF
    echo "database.properties created"
fi

# Create internal.properties to auto-accept license agreement
if [ ! -f "$CONFIG_DIR/internal.properties" ]; then
    echo "Creating internal.properties to accept license agreement..."
    cat > "$CONFIG_DIR/internal.properties" << 'EOF'
# Auto-accept license agreement for non-interactive setup (since TeamCity 2017.2)
teamcity.licenseAgreement.accepted=true
EOF
    echo "internal.properties created"
fi

# Start TeamCity server (exec to replace the shell process)
exec /run-services.sh
