export default async function handler(req, res) {
  if (req.method !== 'POST') {
    res.status(405).json({ error: 'Method not allowed' });
    return;
  }

  const backendBaseUrl = process.env.GO_API_BASE_URL || process.env.NEXT_PUBLIC_GO_API_URL;
  const updateKey = process.env.UPDATE_API_KEY;

  if (!backendBaseUrl || !updateKey) {
    res.status(500).json({ error: 'Update proxy is not configured' });
    return;
  }

  const response = await fetch(`${backendBaseUrl.replace(/\/$/, '')}/api/update/async`, {
    method: 'GET',
    headers: {
      'X-Update-Key': updateKey,
    },
  });

  const text = await response.text();

  res.status(response.status);
  res.setHeader('Content-Type', response.headers.get('content-type') || 'application/json');
  res.send(text);
}