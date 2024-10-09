// This is a mock API for now. In a real application, this would make HTTP requests to your backend.

export interface QueryResponse {
  result: string;
  graph: string;
  error?: string;
}

export async function executeQuery(query: string): Promise<QueryResponse> {
  const response = await fetch('/api/query', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ query }),
  });

  if (!response.ok) {
    const data = await response.json();
    throw new Error(data.error);
  }

  const data = await response.json();
  console.log("API response:", data); // Add this line for debugging

  return data;
}