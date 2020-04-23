{{ $args := parseArgs 3 (joinStr "" "Usage: `[Title]` `[Description]` `[Image URL]` `[Color (optional)]`. You may leave non-optional arguments blank (i.e. `\"\"`) but they __must__ be present.")
    (carg "string" "key")
    (carg "string" "value")
    (carg "string" "image URL")
    (carg "string" "color (optional)")
}}

{{ execCC 3 nil 0 (sdict "Title" ($args.Get 0) "Description" ($args.Get 1) "ImageURL" ($args.Get 2) "Color" ($args.Get 3)) }}

{{ deleteTrigger 0 }}