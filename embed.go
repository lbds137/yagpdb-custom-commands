{{ $args := parseArgs 3 (joinStr "" "Usage: `[Title]` `[Description]` `[Image URL]`. You may leave arguments blank (i.e. `\"\"`) but they __must__ be present.")
    (carg "string" "key")
    (carg "string" "value")
    (carg "string" "image URL")
}}
{{ execCC 3 nil 0 (sdict "Key" ($args.Get 0) "Value" ($args.Get 1) "ImageURL" ($args.Get 2)) }}
{{ deleteTrigger 0 }}