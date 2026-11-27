import { useParams } from "react-router-dom";
import { JobView } from "../JobView";
import { PageTitle } from "../layout";
import { Card } from "../ui";

export function JobPage() {
  const { id = "" } = useParams();
  return (
    <div className="mx-auto max-w-3xl">
      <PageTitle>Progress</PageTitle>
      <Card>
        <JobView id={id} />
      </Card>
    </div>
  );
}
