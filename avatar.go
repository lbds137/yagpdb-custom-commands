{{ $validSizes := cslice "32" "64" "128" "256" "512" "1024" }}
{{ $user := .Member.User }}
{{ $size := "128" }}

{{ $argUser := "" }}
{{ $argSize := "" }}
{{ if eq (len .CmdArgs) 2 }}
    {{ $argUser = userArg (index .CmdArgs 0) }}
    {{ $argSize = index .CmdArgs 1 }}
{{ else if eq (len .CmdArgs) 1 }}
    {{ $arg := index .CmdArgs 0 }}
    {{ $argUser = userArg $arg }}
    {{ $argSize = $arg }}
{{ end }}

{{ if $argUser }}
    {{ $user = $argUser }}
{{ end }}
{{ if in $validSizes $argSize }}
    {{ $size = $argSize }}
{{ end }}
{{ $avatarURL := $user.AvatarURL $size }}

{{ execCC 3 nil 0 (sdict "ImageURL" $avatarURL) }}