{{ $kickReason := "pinging staff in an attempt to expedite the server admission process" }} 
{{ $silent := execAdmin "kick" .User.ID $kickReason }}
{{ joinStr "" "Kicked user **" .User.String " for " $kickReason "!" }} 
 
{{ deleteTrigger 0 }}
{{ deleteResponse 5 }} 