require "json"
require "yaml"

source_path = ARGV.fetch(0)
output_path = ARGV[1]
submission = YAML.safe_load_file(source_path)

abort "Store submission overrides must be a YAML object." unless submission.is_a?(Hash)

notes = submission["NotesForCertification"]
if notes && (!notes.is_a?(String) || notes.length > 2_000)
  abort "NotesForCertification must be a string of at most 2,000 characters."
end

if submission.key?("Listings")
  listings = submission["Listings"]
  abort "Listings must be a YAML object." unless listings.is_a?(Hash)

  if listings.key?("en-us")
    en_us_listing = listings["en-us"]
    abort "Listings.en-us must be a YAML object." unless en_us_listing.is_a?(Hash)

    listing = en_us_listing["BaseListing"]
    if listing
      abort "Listings.en-us.BaseListing must be a YAML object." unless listing.is_a?(Hash)

      release_notes = listing["ReleaseNotes"]
      if release_notes && (!release_notes.is_a?(String) || release_notes.length > 1_500)
        abort "ReleaseNotes must be a string of at most 1,500 characters."
      end

      features = listing["Features"]
      if features
        abort "Features must be a list with at most 20 entries." unless features.is_a?(Array) && features.length <= 20
        abort "Each Features entry must be a string of at most 200 characters." unless features.all? { |feature| feature.is_a?(String) && feature.length <= 200 }
      end
    end
  end
end

File.write(output_path, JSON.generate(submission)) if output_path
