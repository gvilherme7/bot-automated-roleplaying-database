require("firecast.lua");
require("internet.lua");
require("ndb.lua");
require("utils.lua");

local DEFAULT_BASE_URL = "http://localhost:8080"
local DEFAULT_API_KEY = "change_me"
local DEFAULT_AVATAR = "https://encrypted-tbn0.gstatic.com/images?q=tbn:ANd9GcRT49DdfYUXJRo6hPNkbqyVu4nclY4psP34UQ&s"

-- Local-only persistent store (never synced to the table/room), same
-- mechanism used by other plugins (e.g. AfkBot's `NDB.load("afkData.xml")`)
-- to keep the auth token off any data that gets shared with other players.
local config = NDB.load("bardConfig.xml")

local function getAPIKey()
    return config.apiKey or DEFAULT_API_KEY
end

local function getBaseURL()
    return config.baseURL or DEFAULT_BASE_URL
end

local function getAvatarURL()
    return config.avatarURL or DEFAULT_AVATAR
end

local function openConfigWindow()
    local cfgForm = GUI.newForm("frmBardConfig")
    cfgForm:setNodeObject(config)
    cfgForm:show()
end

local function sanitizeQuery(query)
    query = query:match("^%s*(.-)%s*$")
    query = query:gsub("[%c]", "")
    return query
end

local syncQueue = {}
local isSyncing = false

local function sendChat(message, text)
    if message and message.chat then
        local success = pcall(function()
            message.chat:asyncSendStd(text, {
                impersonation = {
                    mode = "character",
                    name = "B.A.R.D",
                    avatar = getAvatarURL()
                }
            })
        end)
        
        -- Fallback if the user doesn't have permission to impersonate or if async fails
        if not success then
            message.chat:enviarMensagem("[B.A.R.D]: " .. text)
        end
    else
        showMessage("[B.A.R.D]: " .. text)
    end
end

Firecast.Messaging.listen("HandleChatCommand", function(message)
    if message.comando == "lore" then
        local rawQuery = message.parametro or ""
        local query = sanitizeQuery(rawQuery)

        if string.len(query) > 500 then
            sendChat(message, "Your question is too long. Max 500 characters.")
            message.response = { handled = true }
            return true
        elseif string.len(query) == 0 then
            sendChat(message, "Please ask a valid question.")
            message.response = { handled = true }
            return true
        end

        local apiKey = getAPIKey()
        local baseURL = getBaseURL()
        local url = baseURL .. "/api/lore"

        -- Echo the question as the user so everyone sees what is being asked
        if message and message.chat then
            message.chat:asyncSendStd(query)
        end

        local safeQueryForJson = query:gsub('"', '\\"')
        local jsonPayload = '{"query": "' .. safeQueryForJson .. '"}'

        -- 1) Send concurrent Acknowledgment Request
        local ackUrl = baseURL .. "/api/acknowledge"
        local ackRequest = Internet.newHTTPRequest("POST", ackUrl)
        ackRequest:setRequestHeader("Authorization", "Bearer " .. apiKey)
        ackRequest:setRequestHeader("Content-Type", "application/json; charset=utf-8")
        ackRequest.onResponse = function()
            if ackRequest.status == 200 then
                local text = ackRequest.responseText
                local _, valStart = text:find('"answer"%s*:%s*"')
                if valStart then
                    local answer = ""
                    local i = valStart + 1
                    while i <= #text do
                        local c = text:sub(i, i)
                        if c == '\\' then
                            local nextC = text:sub(i+1, i+1)
                            if nextC == 'n' then answer = answer .. '\n'
                            elseif nextC == '"' then answer = answer .. '"'
                            elseif nextC == '\\' then answer = answer .. '\\'
                            elseif nextC == 't' then answer = answer .. '\t'
                            elseif nextC == 'r' then answer = answer .. '\r'
                            else answer = answer .. nextC end
                            i = i + 2
                        elseif c == '"' then break
                        else
                            answer = answer .. c
                            i = i + 1
                        end
                    end
                    sendChat(message, answer)
                end
            end
        end
        ackRequest:send(jsonPayload)

        -- 2) Send the actual Lore Search Request
        local request = Internet.newHTTPRequest("POST", url)
        request:setRequestHeader("Authorization", "Bearer " .. apiKey)
        request:setRequestHeader("Content-Type", "application/json; charset=utf-8")

        request.onResponse = function()
            if request.status == 200 then
                local text = request.responseText
                local _, valStart = text:find('"answer"%s*:%s*"')
                
                if valStart then
                    local answer = ""
                    local i = valStart + 1
                    while i <= #text do
                        local c = text:sub(i, i)
                        if c == '\\' then
                            local nextC = text:sub(i+1, i+1)
                            if nextC == 'n' then
                                answer = answer .. '\n'
                            elseif nextC == '"' then
                                answer = answer .. '"'
                            elseif nextC == '\\' then
                                answer = answer .. '\\'
                            elseif nextC == 't' then
                                answer = answer .. '\t'
                            elseif nextC == 'r' then
                                answer = answer .. '\r'
                            else
                                answer = answer .. nextC
                            end
                            i = i + 2
                        elseif c == '"' then
                            break
                        else
                            answer = answer .. c
                            i = i + 1
                        end
                    end
                    sendChat(message, answer)
                else
                    sendChat(message, "Error: Failed to parse the scroll.")
                end
            elseif request.status == 401 then
                sendChat(message, "Error: Unauthorized! Check your API Key.")
            elseif request.status == 400 then
                sendChat(message, "Error: The archives rejected your query syntax.")
            elseif request.status == 500 then
                sendChat(message, "Backend Error: " .. (request.responseText or "Internal Server Error"))
            else
                sendChat(message, "Error: The archives are currently inaccessible (HTTP " .. tostring(request.status) .. ").")
            end
        end

        request.onError = function(errorMsg)
            sendChat(message, "Connection failed: " .. tostring(errorMsg))
        end

        request:send(jsonPayload)

        message.response = { handled = true }
        return true
	elseif message.comando == "lore_config" then
		openConfigWindow()

		message.response = { handled = true }
		return true
	elseif message.comando == "lore_add" then
		local rawQuery = message.parametro or ""
		local query = sanitizeQuery(rawQuery)

		if string.len(query) == 0 then
			sendChat(message, "Please provide the lore you want to add.")
			message.response = { handled = true }
			return true
		end

		local apiKey = getAPIKey()
		local url = getBaseURL() .. "/api/documents"

		sendChat(message, "Adding lore to archives...")

		local request = Internet.newHTTPRequest("POST", url)
		request:setRequestHeader("Authorization", "Bearer " .. apiKey)
		request:setRequestHeader("Content-Type", "application/json; charset=utf-8")

		request.onResponse = function()
			if request.status == 200 then
				sendChat(message, "Lore added successfully!")
			else
				sendChat(message, "Error: Failed to add lore (HTTP " .. tostring(request.status) .. ").")
			end
		end

		request.onError = function(errorMsg)
			sendChat(message, "Connection failed: " .. tostring(errorMsg))
		end

		local safeQueryForJson = query:gsub('"', '\\"')
		local jsonPayload = '{"query": "' .. safeQueryForJson .. '"}'

		request:send(jsonPayload)

		message.response = { handled = true }
		return true

	elseif message.comando == "lore_sync" then
		local mesa = message.room or message.mesa
		if not mesa and message.chat then
			mesa = Firecast.getMesaDe(message.chat)
		end
		
		if not mesa then
			sendChat(message, "Not inside a room.")
			message.response = { handled = true }
			return true
		end

		local apiKey = getAPIKey()

		local function extractAllText(n, depth)
			if depth > 10 then return "" end
			if not n then return "" end
			
			local extracted = ""
			
			if NDB then
				local successAttr, attrs = pcall(NDB.getAttributes, n)
				if successAttr and type(attrs) == "table" then
					for k, v in pairs(attrs) do
						if type(v) == "string" and string.len(v) > 2 then
							local clean = v:gsub("<[^>]+>", " ")
							if string.len(clean) < 10000 then
								extracted = extracted .. clean .. "\n"
							end
						elseif type(v) == "userdata" or type(v) == "table" then
							extracted = extracted .. extractAllText(v, depth + 1)
						end
					end
				end
				
				local successChild, children = pcall(NDB.getChildNodes, n)
				if successChild and type(children) == "table" then
					for i = 1, #children do
						extracted = extracted .. extractAllText(children[i], depth + 1)
					end
				end
				
				if successAttr or successChild then
					return extracted
				end
			end
			
			if type(n) == "table" then
				for k, v in pairs(n) do
					if type(v) == "string" and string.len(v) > 2 then
						local clean = v:gsub("<[^>]+>", " ")
						if string.len(clean) < 10000 then
							extracted = extracted .. clean .. "\n"
						end
					elseif type(v) == "table" or type(v) == "userdata" then
						extracted = extracted .. extractAllText(v, depth + 1)
					end
				end
			end
			
			return extracted
		end

		local function processNext()
			if #syncQueue == 0 then
				sendChat(message, "Library synchronization complete!")
				isSyncing = false
				return
			end

			local queueItem = table.remove(syncQueue, 1)
			local item = queueItem.node
			local itemPath = queueItem.path

			if item.asyncOpenNDB and item.tipo ~= "pasta" and item.tipo ~= "folder" then
				item:asyncOpenNDB():thenDo(function(node)
					if not node then
						setTimeout(processNext, 10)
						return
					end
					
					local itemType = "Text Note"
					if item.tipo == "personagem" or item.tipo == "ficha" then
						itemType = "Character Sheet"
					end
					
					local lowerPath = string.lower(itemPath)
					if lowerPath:find("log") or lowerPath:find("sessão") or lowerPath:find("sessao") then
						itemType = "Session Log"
					end
					
					local titleStr = tostring(item.nome or "Unknown")
					local text = ""
					local successXML, xmlString = pcall(NDB.exportXML, node)
					if successXML and type(xmlString) == "string" then
						for value in xmlString:gmatch('="(.-)"') do
							if string.len(value) > 2 then
								text = text .. " " .. value
							end
						end
						for value in xmlString:gmatch(">(.-)<") do
							if string.len(value) > 2 then
								text = text .. " " .. value
							end
						end
						text = text:gsub("&lt;", "<"):gsub("&gt;", ">"):gsub("&quot;", '"'):gsub("&amp;", "&")
						text = text:gsub("<[^>]+>", " ")
						-- Strip Firecast formatting: color codes, font declarations, encoding headers
						text = text:gsub("%$[0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f]?[0-9A-Fa-f]?", "")
						text = text:gsub("[Rr]oboto%s+txt", "")
						text = text:gsub("[Aa]rial%s+txt", "")
						text = text:gsub("[Vv]erdana%s+txt", "")
						text = text:gsub("1%.0%s+UTF%-8", "")
						text = text:gsub("%d%d/%d%d/%d%d%d%d%s*%-%s*%d%d:%d%d%s*—%s*%S+%s+por%s+%S+", "")
						text = text:gsub("  +", " ")
					else
						text = extractAllText(node, 0)
					end

					if string.len(text) > 50 then
						local request = Internet.newHTTPRequest("POST", getBaseURL() .. "/api/etl/ingest")
						request:setRequestHeader("Authorization", "Bearer " .. apiKey)
						request:setRequestHeader("Content-Type", "application/json; charset=utf-8")
						
						local safeContent = text:gsub("\\", "\\\\"):gsub('"', '\\"'):gsub("\n", "\\n"):gsub("\r", ""):gsub("\t", " ")
						local safeTitle = titleStr:gsub('"', '\\"')
						local safePath = itemPath:gsub('"', '\\"')
						
						local jsonPayload = '{"path": "' .. safePath .. '", "type": "' .. itemType .. '", "title": "' .. safeTitle .. '", "content": "' .. safeContent .. '"}'
						
						request.onResponse = function()
							setTimeout(processNext, 50)
						end
						
						request.onError = function(errorMsg)
							setTimeout(processNext, 50)
						end
						
						request:send(jsonPayload)
					else
						setTimeout(processNext, 10)
					end
				end):onError(function(err)
					setTimeout(processNext, 10)
				end)
			else
				setTimeout(processNext, 10)
			end
		end

		local function enqueueItem(item, path)
			if not item then return end
			
			local newPath
			if path == "" then
				newPath = item.nome or "Unknown"
			else
				newPath = path .. " > " .. tostring(item.nome or "Unknown")
			end
			
			table.insert(syncQueue, { node = item, path = newPath })
			if item.filhos then
				for i = 1, #item.filhos do
					enqueueItem(item.filhos[i], newPath)
				end
			end
		end

		if isSyncing then
			sendChat(message, "A sync is already in progress. Please wait.")
			message.response = { handled = true }
			return true
		end

		sendChat(message, "Synchronizing library sequentially to prevent crashes...")
		
		if mesa.biblioteca and mesa.biblioteca.filhos then
			for i = 1, #mesa.biblioteca.filhos do
				enqueueItem(mesa.biblioteca.filhos[i], "")
			end
		end

		if #syncQueue > 0 then
			isSyncing = true
			sendChat(message, "Found " .. tostring(#syncQueue) .. " items. Beginning ingestion...")
			processNext()
		else
			sendChat(message, "Library is empty.")
		end

		message.response = { handled = true }
		return true
    end
end)

Firecast.Messaging.listen("ListChatCommands", function(message)
    message.response = {
        { comando = "/lore <pergunta>", descricao = "Pergunta algo ao B.A.R.D. sobre a campanha." },
        { comando = "/lore_add <texto>", descricao = "Adiciona um texto livre aos arquivos do B.A.R.D." },
        { comando = "/lore_sync", descricao = "Sincroniza a biblioteca da mesa com o backend do B.A.R.D." },
        { comando = "/lore_config", descricao = "Abre a janela de configuração do B.A.R.D. (backend, token e avatar)." },
    }
end)
