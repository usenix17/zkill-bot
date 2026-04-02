# R2Z2 ‐ Ephemeral Killmail Storage for API Iteration (β)

As zKillboard parses killmails each one is giving a sequence. This sequence provides a means to iterate new killmails as zKill receives, parses, and then posts to the R2Z2 Cloudflare R2 Bucket (S3 compatible).

To discover and iterate these killmails, view a starting sequence `https://r2z2.zkillboard.com/ephemeral/sequence.json` and this will look similar to

```json
{ "sequence": 96088891 }
```

Using this sequence value you can then pull JSON from `https://r2z2.zkillboard.com/ephemeral/96088891.json`. You now have killmail data that includes:

- The `killmail_id` and `hash`
- The raw killmail as seen from `esi`
- zKillboard's metadata included in the `zkb` block
- The unixtime the file was `uploaded_at`
- The killmail's `sequence_id` which will match the file name
- If/when a previous sequence has been updated, it will show in a `sequence_updated` field. (e.g. the killmail was marked post-process as a gank)

About `sequence_id`'s

- `sequence_id`s are strictly increasing
- They are global across all killmails
- They will not be reused
- They are monotonic

## Best Practice

- Obtain a starting sequence
- Iterate and process sequences until a 404 is received, at which point there are no more killmails (yet)
- Sleep at minimum 6 seconds
- Begin iterating sequences again using the latest sequence number in step 2

Following is a pseudocode of one method to iterate the files:

```
sequence = getSequenceFromSequenceJson() // sequence.json
while (true) {
  raw = getThatSequenceFile(sequence) // e.g. 96088891
  if (raw.status == 404) {
    sleep(6000) // try again after 6000ms
  } else {
    doStuffWithIt(raw)
    sleep(100) // sleep 100ms to keep poll rate at 10/s
    sequence++
  }
}
```

## FAQ

### Why did you call it R2Z2?

I was going to call it r2z but I just had to give a respectful nod to a great little droid.

### How often is the `sequence.json` file updated?

Every fifty one killmails a new `sequence.json` is uploaded with the latest `sequence_id`.

### I want to filter what killmails I have to fetch, how do I do that?

If you need to filter by region, alliance, ship type, etc., perform that filtering after retrieving the killmail data. The feed provides an ordered stream of killmails; selection, filtering logic, and any optimization on your side are the responsibility of the consumer.

### Will the JSON structure change?

The JSON structure will be consistent. If there are any changes in the future it will be with the addition of new fields.

### Can you elaborate on `sequence_updated`?

The `sequence_updated` indicates that the file for that sequence has been updated with new information. The sequence has not changed, this just indicates that more metadata has been added. Your code does not need to refetch this file unless it wants the new metadata.

### A really old killmail from years ago came through! I thought this was for only new killmails?

The concept is new killmails to zKillboard. Older killmails can be added to zKillboard at any time!

### The same killmail came through with a new sequence_id! What gives?

This is expected. On occasion zKillboard re-processes killmails for various reasons.

### How long is ephemeral for these sequence files?

The sequence files will last at minimum 24 hours. After this they are expired and removed from the R2 bucket.

### Can I iterate backwards?

Of course! Keep in mind the files are ephemeral. If you're looking for a full dump of all killmails see the API (History) wiki page.

### Is there a rate limit?

Yes. The R2 has a rate limit of 20 requests per second per IP. If you go over this limit you will receive a 429 http status code.

### Why do I have to wait 6 seconds when I receive a 404?

Since late 2008 there has been an average of 1 new killmail every 5.5 seconds. Waiting 6 seconds is the polite way of waiting for the next killmail. Attempting to poll faster will not be of any benefit. Of course, if you prefer, your code can sleep more than 6 seconds. `https://evewho.com` has a 99 second sleep interval.

### What happens if I ignore the 6-second wait?

Polling more aggressively than documented provides no benefit and may result in your access being restricted. If blocked, you will receive HTTP 403 responses and will not be able to retrieve further killmails. To avoid disruption, observe the at minimum 6-second delay after a 404.
