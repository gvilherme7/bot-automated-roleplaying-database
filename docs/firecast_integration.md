# Firecast SDK 3 Integration

This document outlines how the `firecast-plugin` folder implements features from the Firecast SDK 3 to interact with the Go REST API backend.

## Overview

The project has fully transitioned from a Discord bot to a backend service tailored for a custom Firecast plugin. The plugin, packaged as an `.rpk` file (using `module.xml`), leverages Lua 5.4 scripting to parse chat commands and communicate via HTTP to the RAG backend.

## SDK 3 Features Used

### 1. Chat Interception (`Firecast.Messaging.listen`)

The plugin uses the global messaging interceptor to silently capture specific commands inputted by the user in the Firecast chat room.

- **`HandleChatCommand`**: The listener triggers on any chat command. We pattern-match on `message.comando` for commands such as `lore`, `lore_add`, and `lore_sync`.
- **Consumption**: By setting `message.response = { handled = true }` and returning `true`, the plugin consumes the chat message so it doesn't appear as a raw slash command to other users in the room.

### 2. HTTP Requests (`Internet.newHTTPRequest`)

Because the SDK 3 environment is sandboxed, we use `Internet.newHTTPRequest` to securely transmit data between the client and the Go backend.

- **Asynchronous Execution**: `request.onResponse` and `request.onError` callbacks handle the asynchronous responses from the Go backend.
- **Authorization**: The plugin injects an `Authorization: Bearer <API_KEY>` header with each request, securing the endpoints from unauthorized access.
- **Payloads**: Data is sent as `application/json` strings constructed manually (to avoid external dependencies if `utils.jsonEncode` is not available).

### 3. NodeDatabase (NDB)

Firecast manages character sheets and tabletop objects via an NDB (NodeDatabase) tree structure. The plugin directly accesses these nodes for semantic synchronization.

- **Asynchronous NDB Loading**: Using `item:asyncOpenNDB():thenDo(function(node) ... end)`, the plugin asynchronously opens and reads complex character structures (e.g., `background`, `appearance`, `notes`, `traits`) or document annotations.
- **Synchronization**: The text is extracted, sanitized, and batched into HTTP requests sent to the backend's embedding generator, ensuring the Vector database stays in sync with the tabletop's current state.

### 4. Plugin Manifest (`module.xml`)

The `module.xml` defines the metadata for the `.rpk` (RPG Package). It declares the plugin's ID, version, and the entry script (`main.lua`) that bootstrapping the SDK listeners upon load.
