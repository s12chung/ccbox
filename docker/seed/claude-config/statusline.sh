#!/usr/bin/env bash
# Format: [Sonnetʰⁱ*] 15% | 1h 2m • 85% | ●³ᵈ   (model+effort(*=thinking) · context · 5h reset/used · 7d gauge+reset)
input=$(cat)

GRAY='\033[90m'; YELLOW='\033[33m'; RED='\033[31m'; RESET='\033[0m'

pick_color() {
  local value=$1 yellow_at=$2 red_at=$3
  if   [ "$value" -ge "$red_at" ];    then printf '%s' "$RED"
  elif [ "$value" -ge "$yellow_at" ]; then printf '%s' "$YELLOW"
  else printf '%s' "$RESET"; fi
}

pct() {
  local value=$1 yellow_at=$2 red_at=$3 color
  color=$(pick_color "$value" "$yellow_at" "$red_at")
  printf '%b' "${color}${value}%${RESET}"
}

gauge() {
  local v=$1 g color
  if   [ "$v" -ge 100 ]; then g='⊗'; color=$RED
  elif [ "$v" -ge 90 ];  then g='●'; color=$YELLOW
  elif [ "$v" -ge 75 ];  then g='◕'; color=$RESET
  elif [ "$v" -ge 50 ];  then g='◒'; color=$GRAY
  else g='○'; color=$GRAY; fi
  printf '%b' "${color}${g}${RESET}"
}

sup() {  # digits + a-z (no unicode superscript for q) to superscript, for arbitrary effort strings
  echo "$1" | sed 's/0/⁰/g;s/1/¹/g;s/2/²/g;s/3/³/g;s/4/⁴/g;s/5/⁵/g;s/6/⁶/g;s/7/⁷/g;s/8/⁸/g;s/9/⁹/g;
    s/a/ᵃ/g;s/b/ᵇ/g;s/c/ᶜ/g;s/d/ᵈ/g;s/e/ᵉ/g;s/f/ᶠ/g;s/g/ᵍ/g;s/h/ʰ/g;s/i/ⁱ/g;s/j/ʲ/g;s/k/ᵏ/g;s/l/ˡ/g;
    s/m/ᵐ/g;s/n/ⁿ/g;s/o/ᵒ/g;s/p/ᵖ/g;s/r/ʳ/g;s/s/ˢ/g;s/t/ᵗ/g;s/u/ᵘ/g;s/v/ᵛ/g;s/w/ʷ/g;s/x/ˣ/g;s/y/ʸ/g;s/z/ᶻ/g'
}

date_sup() {  # secs-left to largest unit, superscripted
  local secs tok d h m
  secs=$(( ${1:-0} - $(date +%s) ))
  [ "$secs" -le 0 ] && { printf '%b' "${GRAY}·${RESET}"; return; }
  d=$((secs / 86400)); h=$((secs / 3600)); m=$((secs / 60))
  if   [ "$d" -ge 1 ]; then tok="${d}d"
  elif [ "$h" -ge 1 ]; then tok="${h}h"
  else tok="${m}m"; fi
  printf '%b' "${RESET}$(sup "$tok")${RESET}"
}

MODEL=$(echo "$input" | jq -r '.model.display_name // empty')
EFFORT=$(echo "$input" | jq -r '.effort.level // empty')
THINKING=$(echo "$input" | jq -r '.thinking.enabled // false')
model_color() {
  case "$MODEL" in
    "")      printf '%s' "$GRAY" ;;
    Haiku*)  printf '%s' "$RED" ;;
    Sonnet*) printf '%s' "$GRAY" ;;
    *)       printf '%s' "$YELLOW" ;;
  esac
}
effort_color() {
  case "$EFFORT" in
    "")        printf '%s' "$GRAY" ;;
    low)       printf '%s' "$RED" ;;
    medium)    printf '%s' "$GRAY" ;;
    high)      printf '%s' "$GRAY" ;;
    xhigh)     printf '%s' "$RESET" ;;
    max)       printf '%s' "$RED" ;;
    ultracode) printf '%s' "$RED" ;;
    *)         printf '%s' "$RED" ;;
  esac
}
color_rank() {  # GRAY < RESET < YELLOW < RED
  case "$1" in
    "$GRAY")   echo 0 ;;
    "$RESET")  echo 1 ;;
    "$YELLOW") echo 2 ;;
    "$RED")    echo 3 ;;
  esac
}
tag_color() {  # most severe of model_c/effort_c covers the whole [model+effort] tag
  local model_c effort_c
  model_c=$(model_color); effort_c=$(effort_color)
  [ "$(color_rank "$model_c")" -ge "$(color_rank "$effort_c")" ] && printf '%s' "$model_c" || printf '%s' "$effort_c"
}

model_tag() {
  [ -z "$MODEL" ] && printf '%s' '-' && return
  echo "$MODEL" | awk '{print $1}'
}
effort_label() {
  case "$EFFORT" in
    "")        printf '%s' '-' ;;
    low)       printf '%s' 'lo' ;;
    medium)    printf '%s' 'med' ;;
    high)      printf '%s' 'hi' ;;
    xhigh)     printf '%s' 'xhi' ;;
    max)       printf '%s' 'max' ;;
    ultracode) printf '%s' 'ultc' ;;
    *)         printf '%s' "$EFFORT" ;;
  esac
}
effort_tag() {
  local dot=''
  [ "$THINKING" = "true" ] && dot='˙'
  printf '%s' "$(sup "$(effort_label)")${dot}"
}

REMAINING=$(echo "$input" | jq -r '.context_window.remaining_percentage // empty')
context_pct() {
  [ -z "$REMAINING" ] && printf '%b' "${GRAY}-%${RESET}" && return
  pct $(( 100 - ${REMAINING%.*} )) 50 80
}

FIVE_USED=$(echo "$input" | jq -r '.rate_limits.five_hour.used_percentage // empty')
FIVE_RESET=$(echo "$input" | jq -r '.rate_limits.five_hour.resets_at // empty')
five_used() {
  [ -z "$FIVE_USED" ] && printf '%b' "${GRAY}5h-%%${RESET}" && return
  pct "$(printf '%.0f' "$FIVE_USED")" 60 80
}
five_reset() {
  local secs_left h m s
  secs_left=$(( ${FIVE_RESET:-0} - $(date +%s) ))
  [ "$secs_left" -le 0 ] && printf '%b' "${GRAY}5h-reset${RESET}" && return
  h=$((secs_left / 3600)); m=$(((secs_left % 3600) / 60)); s=$((secs_left % 60))
  [ "$h" -gt 0 ] && printf '%dh %dm' "$h" "$m" || printf '%dm %ds' "$m" "$s"
}

WEEK_USED=$(echo "$input" | jq -r '.rate_limits.seven_day.used_percentage // empty')
WEEK_RESET=$(echo "$input" | jq -r '.rate_limits.seven_day.resets_at // empty')
week_used() {  # 7d gauge, gray placeholder when absent
  [ -z "$WEEK_USED" ] && { printf '%b' "${GRAY}-7d-${RESET}"; return; }
  gauge "$(printf '%.0f' "$WEEK_USED")"
}
week_reset() {
  date_sup "$WEEK_RESET"
}

printf '%b\n' "$(tag_color)[$(model_tag)$(effort_tag)]${RESET} $(context_pct) | $(five_reset) • $(five_used) $(week_used)$(week_reset)"
