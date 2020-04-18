{{ $trigger := .Message.Content }}
{{ $tArr := split (slice $trigger 2) "" }}
{{ len $tArr }}