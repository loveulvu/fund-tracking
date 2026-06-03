export default async function handler(req, res) {
  if (req.method !== 'POST') {
    res.setHeader('Allow', ['POST']);
    res.status(405).json({ error: 'Method not allowed' });
    return;
  }

  const backendBaseUrl = process.env.GO_API_BASE_URL || process.env.NEXT_PUBLIC_GO_API_URL;
  const updateKey = process.env.UPDATE_API_KEY;

  if (!backendBaseUrl || !updateKey) {
    res.status(500).json({ error: 'Update proxy is not configured' });
    return;
  }

  try {
    const response = await fetch(`${backendBaseUrl.replace(/\/$/, '')}/api/update/async`, {
      method: 'POST',
      headers: {
        'X-Update-Key': updateKey,
      },
    });

    const text = await response.text();

    res.status(response.status);
    res.setHeader('Content-Type', response.headers.get('content-type') || 'application/json');
    res.send(text);
  } catch (err) {
    console.error('Async update proxy failed:', err);
    res.status(502).json({ error: 'Update proxy request failed' });
  }
}
