# A typical curation session

Load the tools with `EMBYFIN_TOOLSETS=curation`. Each step names the audit that finds the problem and the tool that fixes it. The audits are described in [audits.md](audits.md).

1. **See where the library needs work.** `audit_all` counts what each audit finds.
2. **Match what was never matched.** `audit_missing_metadata problems=provider_id` lists the films never matched to a provider. For each, `item_identify` returns candidates with year and ids. Compare them with the file, then `item_identify_apply candidate=N candidate_ids=…`. The ids pin the choice.
3. **Fix wrong editions and wrong names.** `audit_file_path` finds the wrong editions and the files named for something else. The same two tools fix them.
4. **Fill in artwork.** `audit_missing_metadata problems=poster` lists what has none. `item_artwork` shows the choices and `item_artwork_set` sets one.
5. **Prune and replace.** `audit_duplicates` and `audit_multiple_versions` show what to prune. `audit_runtime`, `audit_provider` and `audit_quality` show what to download again.
6. **Tidy the vocabulary.** `audit_spelling` finds the genres, tags and studios typed several ways, and `metadata_rename` merges each group. `item_edit` with many ids puts the right genre on a whole franchise.
7. **Fill the gaps in series.** `audit_missing_episodes` lists the gaps in each series. With `provider: true` it also lists what each series lacks after its last file.
