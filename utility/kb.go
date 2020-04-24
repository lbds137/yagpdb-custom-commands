{{ $args := parseArgs 1 (joinStr "" "Please enter a key to look up in the knowledge base.")
    (carg "string" "key")
}}
 
{{ $key := $args.Get 0 }}
{{ if $key }}
    {{ execCC 22 nil 0 (sdict "UserID" .Guild.OwnerID "Key" (title $key)) }}
{{ else }}
    {{ execCC 3 nil 0 (sdict "Title" "Info Lookup Failed" "Description" "⚠️ You did not provide a key to look up!") }}
{{ end }}