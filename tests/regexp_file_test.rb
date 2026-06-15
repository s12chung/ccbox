# Validate that allowlist patterns compile as regexes.

require "minitest/autorun"

class RegexpFileTest < Minitest::Test
  ALLOWLIST = File.expand_path("../docker/tinyproxy/allow.txt", __dir__)

  def test_real_allowlist_patterns_all_compile
    assert_empty check_patterns(File.readlines(ALLOWLIST)),
                 "#{ALLOWLIST} has an invalid regex"
  end

  def test_flags_the_one_broken_line
    lines = ['(^|\.)good\.com$', '(^|\.broken'] # 2nd line has an unmatched paren
    errors = check_patterns(lines)

    assert_equal 1, errors.size
    assert_match(/2: bad regex/, errors.first)
  end

  def test_accepts_valid_patterns_comments_and_blanks
    lines = ['(^|\.)good\.com$', "# a comment", ""]

    assert_empty check_patterns(lines)
  end

  private

  def check_patterns(lines)
    errors = []
    lines.each_with_index do |line, i|
      pattern = line.strip
      next if pattern.empty? || pattern.start_with?("#")

      Regexp.new(pattern)
    rescue RegexpError => e
      errors << "#{i + 1}: bad regex: #{pattern} (#{e.message})"
    end
    errors
  end
end
