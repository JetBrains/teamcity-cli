# Reports test results to TeamCity through service messages.
pass() {
  echo "##teamcity[testStarted name='$1' captureStandardOutput='true']"
  echo "##teamcity[testFinished name='$1' duration='$2']"
}

fail() {
  echo "##teamcity[testStarted name='$1' captureStandardOutput='true']"
  echo "##teamcity[testFailed name='$1' message='$3']"
  echo "##teamcity[testFinished name='$1' duration='$2']"
}

# in_list <value> <space-separated list>
in_list() {
  for item in $2; do [ "$item" = "$1" ] && return 0; done
  return 1
}
