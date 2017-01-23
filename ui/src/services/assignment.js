import RPC from './rpc'
import response from './response'

const sv = '/acourse.AssignmentService'

const list = (courseId) => RPC.invoke(sv + '/ListAssignments', { courseId })
  .map(response.assignments)

const create = ({ courseId, title, description }) => RPC.invoke(sv + '/CreateAssignment', {
  courseId,
  title,
  description
})

const update = ({ id, title, description }) => RPC.invoke(sv + '/UpdateAssignment', {
  id,
  title,
  description
})

const open = (assignmentId) => RPC.invoke(sv + '/OpenAssignment', { assignmentId })
const close = (assignmentId) => RPC.invoke(sv + '/CloseAssignment', { assignmentId })

const getUserAssignments = (assignmentIds) => RPC.invoke(sv + '/GetUserAssignments', {
  assignmentIds
})
  .map(response.userAssignments)

const submitUserAssignment = (assignmentId, url) => RPC.invoke(sv + '/SubmitUserAssignment', {
  assignmentId,
  url
})

export default {
  list,
  create,
  update,
  open,
  close,
  getUserAssignments,
  submitUserAssignment
}

